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

// Server implements ShortenerService gRPC API.
type Server struct {
	pb.UnimplementedShortenerServiceServer

	store     model.Store
	urlSvc    *service.URLService
	baseURL   string
	log       *zap.Logger
	secretKey []byte
	aud       *audit.Auditor
}

func New(store model.Store, baseURL string, log *zap.Logger, secretKey []byte) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	if len(secretKey) == 0 {
		secretKey = []byte("very-secret-key")
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &Server{
		store:     store,
		urlSvc:    service.NewURLService(store, log),
		baseURL:   baseURL,
		log:       log,
		secretKey: secretKey,
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

// ShortenURL is an analogue of POST /api/shorten.
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

	shortURL := s.baseURL + "/" + id

	// Same as HTTP: even if URL already existed, associate it with user.
	if err := s.urlSvc.AddUserURL(ctx, userID, id); err != nil {
		s.log.Error("grpc add user url", zap.Error(err))
		return nil, status.Error(codes.Internal, "storage error")
	}

	s.publish(ctx, audit.Event{
		TS:     time.Now().Unix(),
		Action: audit.ActionShorten,
		UserID: userID,
		URL:    original,
	})

	if existed {
		// HTTP returns 409 with shortURL in body; in gRPC use AlreadyExists with shortURL in message.
		return nil, status.Error(codes.AlreadyExists, shortURL)
	}

	return &pb.URLShortenResponse{Result: shortURL}, nil
}

// ExpandURL is an analogue of GET /{id}.
func (s *Server) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (*pb.URLExpandResponse, error) {
	id := strings.TrimSpace(req.GetId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "empty id")
	}

	original, ok, err := s.urlSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, model.ErrDeleted) {
			// HTTP returns 410 Gone.
			return nil, status.Error(codes.FailedPrecondition, "url deleted")
		}
		s.log.Error("grpc get error", zap.Error(err))
		return nil, status.Error(codes.Internal, "storage error")
	}
	if !ok {
		// HTTP handler returns 400 for unknown id.
		return nil, status.Error(codes.InvalidArgument, "id not found")
	}

	uid, _ := s.userIDFromAuth(ctx)

	s.publish(ctx, audit.Event{
		TS:     time.Now().Unix(),
		Action: audit.ActionFollow,
		UserID: uid,
		URL:    original,
	})

	return &pb.URLExpandResponse{Result: original}, nil
}

// ListUserURLs is an analogue of GET /api/user/urls.
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

	out := make([]*pb.URLData, 0, len(items))
	for _, it := range items {
		out = append(out, &pb.URLData{
			ShortUrl:    s.baseURL + "/" + it.ID,
			OriginalUrl: it.OriginalURL,
		})
	}

	return &pb.UserURLsResponse{Url: out}, nil
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

// signUserID creates token value compatible with HTTP cookie signing in this repo.
// Format: base64(userID:hex(hmac_signature))
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

// extractTokenFromMD reads and normalises the authorization token from gRPC incoming metadata.
// Returns the raw token string and true if present, or empty string and false otherwise.
func extractTokenFromMD(ctx context.Context) (string, bool) {
	md, _ := metadata.FromIncomingContext(ctx)
	vals := md.Get(authHeaderKey)
	if len(vals) == 0 || strings.TrimSpace(vals[0]) == "" {
		return "", false
	}
	auth := strings.TrimSpace(vals[0])
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		auth = strings.TrimSpace(auth[len("bearer "):])
	}
	return auth, true
}

// getOrCreateUser reads token from incoming metadata. If missing -> creates a new token.
// Returns: (userID, token, createdNow, error)
func (s *Server) getOrCreateUser(ctx context.Context) (string, string, bool, error) {
	auth, ok := extractTokenFromMD(ctx)
	if !ok {
		uid := s.newUserID()
		tok := s.signUserID(uid)
		return uid, tok, true, nil
	}

	uid, valid := s.parseUserID(auth)
	if !valid || uid == "" {
		// close to HTTP behaviour could be "create new", but explicit auth header usually means client expects auth.
		return "", "", false, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	return uid, auth, false, nil
}

func (s *Server) userIDFromAuth(ctx context.Context) (string, bool) {
	auth, ok := extractTokenFromMD(ctx)
	if !ok {
		return "", false
	}
	uid, valid := s.parseUserID(auth)
	if !valid || uid == "" {
		return "", false
	}
	return uid, true
}
