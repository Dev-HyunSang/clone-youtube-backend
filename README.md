# clone-youtube-backend

YouTube 클론 백엔드. Fiber + GORM(PostgreSQL) + Redis + MinIO 기반이다. 현재 회원 인증, 이메일 인증, 영상 업로드, HLS 렌디션 생성 및 진행률 조회까지 구현되어 있다. 영상 메타데이터 CRUD/공개 조회/검색, 댓글, 좋아요, 관리자 기능은 아직 없다.

## 구현된 기능

- 회원가입·로그인: 이메일 중복을 확인하고 비밀번호를 bcrypt로 해시해 저장한다.
- JWT 세션 관리: access token(15분)과 refresh token(7일)을 분리 발급한다. Redis에 세션을 저장하고, refresh token 회전·재사용 탐지, 현재 기기/전체 기기 로그아웃을 지원한다.
- 이메일 인증: SMTP로 6자리 인증 코드를 발송하며, 코드는 5분 후 만료되고 한 번만 사용할 수 있다.
- 영상 업로드: 인증한 사용자가 `Content-Length`와 raw 영상 body를 전송하면 서버가 스트리밍으로 저장한다. 업로드 진행률과 예상 남은 시간을 조회하는 엔드포인트도 구현되어 있다.
- 원본 저장 및 메타데이터: `ffprobe`로 해상도·재생 시간을 추출하고 원본 영상(raw body)을 MinIO의 `.mp4` 키에 저장한다. 업로드 상태와 메타데이터는 PostgreSQL에 기록한다.
- HLS 변환: 업로드 후 asynq/Redis 백그라운드 작업이 원본 해상도를 넘지 않는 2160p·1440p·1080p·720p·480p·360p HLS 렌디션을 생성해 MinIO에 저장한다. 렌디션별 상태와 진행률을 조회할 수 있다.

## 실행

```bash
make docker-dev        # postgres, redis 컨테이너 기동 (.env.dev 사용)
go run ./cmd/gocore     # 서버 실행 (:3000)
```

`.env.sample`을 참고해 `.env.dev`를 작성한다. 필수 환경변수는 PostgreSQL(`POSTGRES_*`), Redis(`REDIS_*`), JWT(`JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`), MinIO(`MINIO_ENDPOINT`, `MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD`, `MINIO_BUCKET`, `MINIO_USE_SSL`)다. `UPLOAD_DIR`을 설정하면 업로드 임시 경로를 바꿀 수 있고, 생략하면 `./uploads`를 사용한다.
이메일 인증 발송에는 별도로 `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`이 필요 (plain auth, TLS 없음).

`ffmpeg`와 `ffprobe`가 서버 실행 환경의 `PATH`에 있어야 영상 메타데이터 추출과 HLS 변환이 동작한다.

## API (`/api/v1`)

요청/응답 스펙은 [API_DOCS.md](API_DOCS.md) 참고.

- `POST /users/signup`, `/users/signin`, `/users/refresh` — 회원가입/로그인/토큰 갱신
- `POST /users/logout`, `/users/logout-all` — 인증 필요, 세션 삭제
- `POST /verify/send`, `/verify/verify` — 이메일 인증 코드(6자리, 5분 만료) 발송/검증
- `POST /videos/upload` — 인증 필요, raw 영상 스트리밍 업로드 및 원본 MinIO 저장
- `GET /videos/upload/:id/progress` — 인증 필요, 업로드 진행률·예상 남은 시간 폴링
- `GET /videos/:id/renditions` — 인증 필요, HLS 렌디션 상태·변환 진행률 폴링

## 디렉토리 구조

```
cmd/gocore/            # main. fiber 라우터 초기화 + DB/Redis 연결 + AutoMigrate
internal/
├── auth/               # JWT 발급/검증 (access 15m + refresh 7d, jti 재사용 탐지)
├── config/              # 환경변수 로딩 (godotenv)
├── db/                  # PostgreSQL, Redis, MinIO, asynq 연결
├── domain/              # User, EmailVerification, Video, VideoRendition GORM 모델
├── dto/
│   ├── request/            # 요청 바인딩 struct
│   └── response/            # 응답 struct
├── handler/              # user, email verification, video upload/progress/renditions
├── media/                # ffprobe/ffmpeg HLS 변환 + 진행률 추적 (in-memory, 단일 프로세스)
├── middleware/
│   ├── middleware.go       # logger/recover 등록
│   ├── auth.go               # Bearer JWT 검증 + Redis 세션 확인
│   ├── admin.go               # 관리자 권한 체크 (미구현, 스텁)
│   └── request_id.go           # 요청 ID 부여 (미구현, 스텁)
├── repository/
│   ├── postgressql/       # 사용자·이메일 인증·영상·렌디션 저장소
│   └── redis/             # 세션 저장소, HLS 렌디션 asynq 태스크
├── service/              # 이메일 발송 (SMTP)
└── usecase/               # signup/signin/refresh/logout, 이메일 인증 코드 흐름
```

레이어 컨벤션(핸들러는 얇게, DTO 방향 분리, 인증 로직은 auth에만 등)은
`.claude/skills/go-layered-structure/SKILL.md`에 정리되어 있으니 새 기능 추가 시 참고.

## 스택

- Web: Fiber v2
- DB: PostgreSQL (GORM)
- Cache/세션: Redis
- 큐/워커: asynq + Redis (프로세스 내 HLS 렌디션 워커, 동시성 2)
- 인증: JWT (golang-jwt/v5)
- 미디어: ffmpeg/ffprobe (외부 바이너리 shell-out)
