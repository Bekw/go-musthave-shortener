# go-musthave-shortener-tpl

Шаблон репозитория для трека «Сервис сокращения URL».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-shortener-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**


### Iter17 pprof diff

go test ./cmd/shortener -run=^$ -bench=Benchmark -benchmem -memprofile .\profiles\result.pprof
goos: windows
goarch: amd64
pkg: github.com/Bekw/go-musthave-shortener/cmd/shortener
cpu: 11th Gen Intel(R) Core(TM) i5-1135G7 @ 2.40GHz
BenchmarkPOST_Plain-8             161180              7068 ns/op            8738 B/op         50 allocs/op
BenchmarkPOST_JSON-8              124328              8309 ns/op            9215 B/op         57 allocs/op
BenchmarkGET_Redirect-8           274708              4026 ns/op            7017 B/op         27 allocs/op
PASS
ok      github.com/Bekw/go-musthave-shortener/cmd/shortener     3.938s

go tool pprof -top -diff_base=.\profiles\base.pprof .\profiles\result.pprof
File: shortener.test.exe
Build ID: C:\Users\b.konyrbayev\Desktop\goyandex\go-musthave-shortener\shortener.test.exe2026-01-26 16:21:44.5715116 +0500 +05
Type: alloc_space
Time: 2026-01-26 16:23:00 +05
Showing nodes accounting for 8.13GB, 96.80% of 8.39GB total
Dropped 79 nodes (cum <= 0.04GB)
      flat  flat%   sum%        cum   cum%
    4.30GB 51.23% 51.23%     4.30GB 51.23%  bufio.NewReaderSize (inline)
    0.68GB  8.07% 59.30%     0.68GB  8.07%  net/http.(*Request).WithContext (inline)
    0.57GB  6.82% 66.12%     0.57GB  6.82%  net/textproto.MIMEHeader.Set (inline)
    0.45GB  5.32% 71.44%     0.45GB  5.32%  net/http.Header.Clone (inline)
    0.34GB  4.09% 75.53%     0.55GB  6.59%  net/http.readRequest
    0.17GB  2.05% 77.58%     0.17GB  2.05%  net/url.parse
    0.16GB  1.92% 79.50%     0.25GB  2.96%  net/http/httptest.(*ResponseRecorder).Result
    0.15GB  1.82% 81.32%     0.15GB  1.82%  net/http/httptest.NewRecorder (inline)
    0.15GB  1.79% 83.11%     0.15GB  1.79%  net/http.(*Request).SetPathValue (inline)
    0.15GB  1.73% 84.84%     0.15GB  1.73%  io.ReadAll
    0.14GB  1.66% 86.50%     0.14GB  1.66%  crypto/internal/fips140/sha256.New (inline)
    0.12GB  1.45% 87.95%     0.12GB  1.45%  encoding/json.(*Decoder).refill
    0.11GB  1.29% 89.24%     0.25GB  2.95%  crypto/internal/fips140/hmac.New[go.shape.interface { BlockSize int; Reset; Size int; Sum []uint8; Write  }]
    0.11GB  1.26% 90.49%     0.11GB  1.26%  net/http.readCookies
    0.09GB  1.05% 91.55%     0.10GB  1.15%  fmt.Sprintf
    0.08GB  0.96% 92.51%     0.08GB  0.96%  encoding/json.NewDecoder (inline)
    0.06GB  0.76% 93.26%     0.06GB  0.76%  bytes.NewReader (inline)
    0.06GB  0.67% 93.93%     0.06GB  0.67%  context.WithValue
    0.05GB  0.56% 94.50%     0.05GB  0.56%  net/textproto.readMIMEHeader
    0.04GB  0.54% 95.03%     0.77GB  9.20%  github.com/Bekw/go-musthave-shortener/internal/handler.(*Handler).PostJSONHandler
    0.04GB   0.5% 95.53%     0.05GB  0.62%  net/url.(*URL).JoinPath
    0.04GB  0.45% 95.99%     0.35GB  4.21%  github.com/Bekw/go-musthave-shortener/internal/handler.(*Handler).parseUserID
    0.03GB  0.31% 96.30%     5.25GB 62.49%  net/http/httptest.NewRequestWithContext
    0.02GB  0.26% 96.56%     0.71GB  8.40%  github.com/Bekw/go-musthave-shortener/internal/handler.(*Handler).PostHandler
    0.02GB  0.24% 96.80%     0.10GB  1.16%  net/http.(*Request).AddCookie
         0     0% 96.80%     4.30GB 51.23%  bufio.NewReader (inline)
         0     0% 96.80%     0.25GB  2.95%  crypto/hmac.New
         0     0% 96.80%     0.14GB  1.66%  crypto/hmac.New.UnwrapNew[go.shape.interface { BlockSize int; Reset; Size int; Sum []uint8; Write  }].func1
         0     0% 96.80%     0.14GB  1.66%  crypto/sha256.New
         0     0% 96.80%     0.15GB  1.73%  encoding/json.(*Decoder).Decode
         0     0% 96.80%     0.12GB  1.48%  encoding/json.(*Decoder).readValue
         0     0% 96.80%     3.47GB 41.38%  github.com/Bekw/go-musthave-shortener/cmd/shortener.BenchmarkGET_Redirect
         0     0% 96.80%     2.34GB 27.86%  github.com/Bekw/go-musthave-shortener/cmd/shortener.BenchmarkPOST_JSON
         0     0% 96.80%     2.58GB 30.69%  github.com/Bekw/go-musthave-shortener/cmd/shortener.BenchmarkPOST_Plain
         0     0% 96.80%     0.40GB  4.72%  github.com/Bekw/go-musthave-shortener/internal/handler.(*Handler).GetHandler
         0     0% 96.80%     0.46GB  5.46%  github.com/Bekw/go-musthave-shortener/internal/handler.(*Handler).ensureUserID
         0     0% 96.80%     0.46GB  5.46%  github.com/Bekw/go-musthave-shortener/internal/handler.(*Handler).readUserID
         0     0% 96.80%     2.43GB 28.99%  github.com/go-chi/chi/v5.(*Mux).ServeHTTP
         0     0% 96.80%     2.02GB 24.12%  github.com/go-chi/chi/v5.(*Mux).routeHTTP
         0     0% 96.80%     0.15GB  1.79%  github.com/go-chi/chi/v5.setPathValue
         0     0% 96.80%     0.11GB  1.26%  net/http.(*Request).Cookie (inline)
         0     0% 96.80%     2.02GB 24.12%  net/http.HandlerFunc.ServeHTTP
         0     0% 96.80%     0.57GB  6.82%  net/http.Header.Set (inline)
         0     0% 96.80%     0.55GB  6.59%  net/http.ReadRequest
         0     0% 96.80%     0.45GB  5.32%  net/http/httptest.(*ResponseRecorder).WriteHeader
         0     0% 96.80%     5.25GB 62.49%  net/http/httptest.NewRequest (inline)
         0     0% 96.80%     0.05GB  0.56%  net/textproto.(*Reader).ReadMIMEHeader (inline)
         0     0% 96.80%     0.10GB  1.16%  net/url.JoinPath
         0     0% 96.80%     0.14GB  1.64%  net/url.ParseRequestURI
         0     0% 96.80%     8.39GB 99.94%  testing.(*B).launch
         0     0% 96.80%     8.39GB 99.94%  testing.(*B).runN