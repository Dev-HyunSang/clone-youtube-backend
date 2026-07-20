# clone-youtube-backend

YouTube 클론 백엔드. Fiber + GORM(PostgreSQL) + Redis 기반. 인증(JWT 발급/갱신/세션), 이메일 인증, 영상 업로드+ffmpeg 트랜스코딩이 동작하는 상태이고, 영상 메타데이터 CRUD/조회/검색/댓글/좋아요/관리자 기능은 아직 없다.

## 실행

```bash
make docker-dev        # postgres, redis 컨테이너 기동 (.env.dev 사용)
go run ./cmd/gocore     # 서버 실행 (:3000)
```

`.env.dev`(DB 연결도 이 키를 그대로 씀, 대문자): `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`.
이메일 인증 발송에는 별도로 `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`이 필요 (plain auth, TLS 없음).

## API (`/api/v1`)

요청/응답 스펙은 [API_DOCS.md](API_DOCS.md) 참고.

- `POST /users/signup`, `/users/signin`, `/users/refresh` — 회원가입/로그인/토큰 갱신
- `POST /users/logout`, `/users/logout-all` — 인증 필요, 세션 삭제
- `POST /verify/send`, `/verify/verify` — 이메일 인증 코드(6자리, 5분 만료) 발송/검증
- `POST /videos/upload`, `GET /videos/upload/:id/progress` — 인증 필요, 스트리밍 업로드 + ffmpeg 트랜스코딩 진행률 폴링

## 디렉토리 구조

```
cmd/gocore/            # main. fiber 라우터 초기화 + DB/Redis 연결 + AutoMigrate
internal/
├── auth/               # JWT 발급/검증 (access 15m + refresh 7d, jti 재사용 탐지)
├── config/              # 환경변수 로딩 (godotenv)
├── db/                   # gorm PostgreSQL + Redis 커넥션
├── domain/               # User GORM 모델
├── dto/
│   ├── request/            # 요청 바인딩 struct
│   └── response/            # 응답 struct
├── handler/              # user(signup/signin/refresh/logout), video(upload/progress)
├── media/                # ffprobe/ffmpeg 트랜스코딩 + 업로드 진행률 추적 (in-memory, 단일 프로세스)
├── middleware/
│   ├── middleware.go       # logger/recover 등록
│   ├── auth.go               # Bearer JWT 검증 + Redis 세션 확인
│   ├── admin.go               # 관리자 권한 체크 (미구현, 스텁)
│   └── request_id.go           # 요청 ID 부여 (미구현, 스텁)
├── repository/
│   ├── postgressql/        # 유저 Create/FindByEmail
│   └── redis/                # 세션 저장소, 영상 리사이즈 asynq 태스크 생성(워커 미등록, 소비 안 됨)
├── service/              # 이메일 발송 (SMTP)
└── usecase/               # signup/signin/refresh/logout, 이메일 인증 코드 흐름
```

레이어 컨벤션(핸들러는 얇게, DTO 방향 분리, 인증 로직은 auth에만 등)은
`.claude/skills/go-layered-structure/SKILL.md`에 정리되어 있으니 새 기능 추가 시 참고.

## 스택

- Web: Fiber v2
- DB: PostgreSQL (GORM)
- Cache/세션: Redis
- 큐: asynq (영상 리사이즈 태스크 정의만 있고 워커 미구현)
- 인증: JWT (golang-jwt/v5)
- 미디어: ffmpeg/ffprobe (외부 바이너리 shell-out)
