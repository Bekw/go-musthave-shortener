package grpcapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"github.com/Bekw/go-musthave-shortener/internal/audit"
	"github.com/Bekw/go-musthave-shortener/internal/grpcapi/pb"
	"github.com/Bekw/go-musthave-shortener/internal/model"
	"github.com/Bekw/go-musthave-shortener/internal/service"
)

const authHeaderKey = "authorization"

type Server struct {
	pb.UnimplementedShortenerServiceServer

	store     model.Store
	urlSvc    *service.URLService
	baseURL   string
	log       *zap.Logger
	secretKey []byte
	aud       *audit.Auditor
}

func New(store model.Store, baseURL string, log *zap.Logger) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &Server{
		store:     store,
		urlSvc:    service.NewURLService(store, log),
		baseURL:   baseURL,
		log:       log,
		secretKey: []byte("very-secret-key"),
	}
}

func (s *Server) SetAuditor(a *audit.Auditor) { s.aud = a }

func (s *Server) Shutdown(ctx context.Context) error {
	if s.urlSvc == nil {
		return nil
	}
	return s.urlSvc.Shutdown(ctx)
}

func (s *Server) Register(grpcServer *grpc.Server) {
	pb.RegisterShortenerServiceServer(grpcServer, s)
}

func (s *Server) ShortenURL(ctx context.Context, req *pb.URLShortenRequest) (*pb.URLShortenResponse, error) {
	original := strings.TrimSpace(req.GetUrl())
	if original == "" {
		return nil, status.Error(codes.InvalidArgument, "empty url")
	}

	userID, token, created, err := s.getOrCreateUser(ctx)
	if err != nil {
		return nil, err
	}
	if created {
		if err := grpc.SetHeader(ctx, metadata.Pairs(authHeaderKey, token)); err != nil {
			s.log.Warn("grpc set header failed", zap.Error(err))
		}
	}

	id, existed, err := s.urlSvc.SaveWithRetries(ctx, original, 5)
	if err != nil {
		s.log.Error("grpc save error", zap.Error(err))
		return nil, status.Error(codes.Internal, "storage error")
	}

	if err := s.urlSvc.AddUserURL(ctx, userID, id); err != nil {
		s.log.Error("grpc add user url", zap.Error(err))
		return nil, status.Error(codes.Internal, "storage error")
	}

	shortURL := s.baseURL + "/" + id

	s.publish(ctx, audit.Event{
		TS:     time.Now().Unix(),
		Action: audit.ActionShorten,
		UserID: userID,
		URL:    original,
	})

	if existed {
		return nil, status.Error(codes.AlreadyExists, shortURL)
	}

	return &pb.URLShortenResponse{Result: shortURL}, nil
}

func (s *Server) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (*pb.URLExpandResponse, error) {
	id := strings.TrimSpace(req.GetId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "empty id")
	}

	original, ok, err := s.urlSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, model.ErrDeleted) {
			return nil, status.Error(codes.FailedPrecondition, "url deleted")
		}
		s.log.Error("grpc get error", zap.Error(err))
		return nil, status.Error(codes.Internal, "storage error")
	}
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "id not found")
	}

	uid, _ := s.userIDFromAuth(ctx)

	s.publish(ctx, audit.Event{
		TS:     time.Now().Unix(),
		Action: audit.ActionFollow,
		UserID: uid,
		URL:    original,
	})

	return &pb.URLExpandResponse{Url: original}, nil
}

func (s *Server) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*pb.UserURLsResponse, error) {
	userID, token, created, err := s.getOrCreateUser(ctx)
	if err != nil {
		return nil, err
	}
	if created {
		if err := grpc.SetHeader(ctx, metadata.Pairs(authHeaderKey, token)); err != nil {
			s.log.Warn("grpc set header failed", zap.Error(err))
		}
	}

	items, err := s.urlSvc.GetUserURLs(ctx, userID)
	if err != nil {
		s.log.Error("grpc get user urls", zap.Error(err))
		return nil, status.Error(codes.Internal, "storage error")
	}

	out := make([]*pb.UserURL, 0, len(items))
	for _, it := range items {
		out = append(out, &pb.UserURL{
			ShortUrl:    s.baseURL + "/" + it.ID,
			OriginalUrl: it.OriginalURL,
		})
	}

	return &pb.UserURLsResponse{Items: out}, nil
}

func (s *Server) publish(ctx context.Context, e audit.Event) {
	if s.aud == nil {
		return
	}
	s.aud.Publish(ctx, e)
}

func (s *Server) newUserID() string {
	return service.GenerateID() + service.GenerateID()
}

func (s *Server) signUserID(id string) string {
	mac := hmac.New(sha256.New, s.secretKey)
	_, _ = mac.Write([]byte(id))
	sig := mac.Sum(nil)

	hexSig := make([]byte, hex.EncodedLen(len(sig)))
	hex.Encode(hexSig, sig)

	payload := make([]byte, len(id)+1+len(hexSig))
	copy(payload, id)
	payload[len(id)] = ':'
	copy(payload[len(id)+1:], hexSig)

	out := make([]byte, base64.URLEncoding.EncodedLen(len(payload)))
	base64.URLEncoding.Encode(out, payload)
	return string(out)
}

func (s *Server) parseUserID(value string) (string, bool) {
	data, err := base64.URLEncoding.DecodeString(value)
	if err != nil {
		return "", false
	}

	i := bytes.IndexByte(data, ':')
	if i <= 0 || i >= len(data)-1 {
		return "", false
	}

	idBytes := data[:i]
	sigHex := data[i+1:]

	sigBytes := make([]byte, hex.DecodedLen(len(sigHex)))
	n, err := hex.Decode(sigBytes, sigHex)
	if err != nil {
		return "", false
	}
	sigBytes = sigBytes[:n]

	mac := hmac.New(sha256.New, s.secretKey)
	_, _ = mac.Write(idBytes)
	expected := mac.Sum(nil)

	if !hmac.Equal(sigBytes, expected) {
		return "", false
	}

	return string(idBytes), true
}

func (s *Server) getOrCreateUser(ctx context.Context) (string, string, bool, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	vals := md.Get(authHeaderKey)
	if len(vals) == 0 || strings.TrimSpace(vals[0]) == "" {
		uid := s.newUserID()
		tok := s.signUserID(uid)
		return uid, tok, true, nil
	}

	auth := strings.TrimSpace(vals[0])
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		auth = strings.TrimSpace(auth[len("bearer "):])
	}
	uid, ok := s.parseUserID(auth)
	if !ok || uid == "" {
		return "", "", false, status.Error(codes.Unauthenticated, "invalid authorization")
	}
	return uid, auth, false, nil
}

func (s *Server) userIDFromAuth(ctx context.Context) (string, bool) {
	md, _ := metadata.FromIncomingContext(ctx)
	vals := md.Get(authHeaderKey)
	if len(vals) == 0 || strings.TrimSpace(vals[0]) == "" {
		return "", false
	}
	auth := strings.TrimSpace(vals[0])
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		auth = strings.TrimSpace(auth[len("bearer "):])
	}
	uid, ok := s.parseUserID(auth)
	if !ok || uid == "" {
		return "", false
	}
	return uid, true
}
