# API 문서

Base URL: `/api/v1`

인증이 필요한 엔드포인트는 `Authorization: Bearer <access_token>` 헤더 필요.
에러 응답은 공통으로 `{"error": "메시지"}` 형식.

## Users

### POST /users/signup

회원가입.

Request body:

```json
{
  "email": "user@example.com",
  "password": "at-least-8-chars",
  "first_name": "길동",
  "last_name": "홍",
  "nickname": "hong",
}
```

`created_at`, `updated_at`은 필수(RFC3339), `deleted_at`은 있으면 RFC3339 형식.

- `201` — 가입 성공, `UserResponse` 반환
  ```json
  {
    "id": "uuid",
    "email": "user@example.com",
    "first_name": "길동",
    "last_name": "홍",
    "nickname": "hong",
    "created_at": "2026-07-17T00:00:00Z"
  }
  ```
- `400` — body 파싱/유효성 실패
- `409` — 이미 존재하는 이메일
- `500` — 서버 오류

### POST /users/signin

로그인.

Request body:

```json
{ "email": "user@example.com", "password": "..." }
```

- `200` — `SignInResponse`
  ```json
  {
    "access_token": "...",
    "refresh_token": "...",
    "user": { "id": "...", "email": "...", "first_name": "...", "last_name": "...", "nickname": "...", "created_at": "..." }
  }
  ```
- `400` — body 파싱 실패
- `401` — 이메일/비밀번호 불일치
- `500` — 서버 오류

### POST /users/refresh

refresh token으로 access/refresh token 재발급 (rotation). 재사용된(이미 로테이션된) refresh token이 오면 해당 세션 전체를 무효화.

Request body:

```json
{ "refresh_token": "..." }
```

- `200` — `RefreshTokenResponse` (`access_token`, `refresh_token` 새로 발급)
- `400` — body 파싱 실패
- `401` — 유효하지 않거나 재사용된 refresh token
- `500` — 서버 오류

### POST /users/logout

🔒 인증 필요. 현재 세션 로그아웃.

- `200` — `{"message": "로그아웃되었습니다"}`
- `401` — 인증 토큰 없음/무효/로그아웃된 세션
- `500` — 서버 오류

### POST /users/logout-all

🔒 인증 필요. 해당 유저의 모든 세션 로그아웃.

- `200` — `{"message": "모든 기기에서 로그아웃되었습니다"}`
- `401` — 인증 실패
- `500` — 서버 오류

## Email Verification

### POST /verify/send

인증 코드(6자리, 5분 만료) 이메일 발송.

Request body:

```json
{ "email": "user@example.com" }
```

- `200` — `{"message": "인증 코드를 발송했습니다"}`
- `400` — body 파싱 실패
- `500` — 서버 오류 (SMTP 등)

### POST /verify/verify

인증 코드 검증.

Request body:

```json
{ "email": "user@example.com", "code": "123456" }
```

- `200` — `{"message": "인증되었습니다"}`
- `400` — body 파싱 실패, 코드 없음/만료/불일치, 이미 인증됨
- `500` — 서버 오류

## Videos

🔒 인증 필요.

### POST /videos/upload

영상 바이너리를 바로 업로드하는 단일 엔드포인트. 서버가 id를 임의로 생성해 `videos` 테이블에 `status: uploading`으로 기록한 뒤, 요청 body를 스트리밍으로 로컬에 저장하며 진행률을 추적(이때부터 GET .../progress 폴링 가능). 저장이 끝나면 ffprobe로 원본 메타데이터(가로/세로/길이)를 추출하고 MinIO 버킷(`MINIO_BUCKET`)에 `<id>.mp4`로 업로드, 완료되면 로컬 임시 파일 삭제. 메타데이터와 최종 상태(`completed`/`failed`)는 `videos` 테이블에 반영.

원본 업로드가 끝나면 HLS 렌디션 생성은 백그라운드 작업(asynq, Redis 큐)으로 큐잉만 하고 바로 응답. 워커가 MinIO에서 원본을 다시 받아와 원본 세로 해상도 이하인 것만 [2160p(4K), 1440p(2K), 1080p, 720p, 480p, 360p] 중에서 골라(즉 원본이 4K면 6단계 전부, 원본이 1080p면 1080p 이하만) 각각 ffmpeg로 트랜스코딩한 뒤 MinIO에 `<videoId>/<height>p/playlist.m3u8` + `<videoId>/<height>p/segment_NNN.ts` 형태로 업로드. 렌디션별 결과는 `video_renditions` 테이블에 한 행씩(`video_id`, `width`, `height`, `playlist_key`, `status`) 기록되고, 진행 상황은 아래 GET .../renditions로 조회. 개별 렌디션이 실패해도 원본 업로드 자체는 실패 처리하지 않고 해당 렌디션 행만 `status: failed`로 남음.

- 필수: `Content-Length` 헤더
- body: 영상 바이너리 (raw)

- `201` — `UploadProgressResponse` (서버가 생성한 id 포함)
  ```json
  { "id": "server-generated-uuid", "status": "completed", "progress": 100 }
  ```
- `400` — `Content-Length` 누락
- `500` — DB 기록/저장/업로드 실패 (저장 이후 실패면 `videos.status`는 `failed`로 기록됨)

### GET /videos/upload/:id/progress

업로드 진행률 조회 (폴링용). `:id`는 위 POST 응답의 `id`.

- `200` — `UploadProgressResponse`
  ```json
  {
    "id": "upload-id",
    "status": "uploading | completed | failed",
    "progress": 42,
    "estimated_minutes_left": 1.5,
    "error": ""
  }
  ```
- `404` — 존재하지 않는 upload id (아직 업로드 요청이 시작되지 않았거나 서버 재시작 등으로 in-memory 진행률 정보가 없음)

### GET /videos/:id/renditions

HLS 렌디션 처리 상황 조회 (백그라운드 처리라 업로드 응답만으론 알 수 없어서 별도 폴링용). `:id`는 업로드 응답의 `id`.

- `200` — `VideoRenditionsResponse`
  ```json
  {
    "video_id": "upload-id",
    "renditions": [
      { "height": 1080, "width": 1920, "playlist_key": "upload-id/1080p/playlist.m3u8", "status": "completed", "progress": 100 },
      { "height": 720, "width": 1280, "playlist_key": "upload-id/720p/playlist.m3u8", "status": "processing", "progress": 42 },
      { "height": 480, "width": 0, "playlist_key": "", "status": "failed", "progress": 0 }
    ]
  }
  ```
  `progress`(0~100)는 `status: processing`인 동안 ffmpeg 인코딩 진행률을 실시간으로 반영, `completed`는 100, 그 외(대기/실패)는 0. 서버 재시작 시 진행 중이던 항목의 진행률은 초기화됨(in-memory).
  아직 워커가 시작 전이면 `renditions`는 빈 배열.

## 인증 토큰

- access token: 15분 만료, `JWT_ACCESS_SECRET`으로 서명
- refresh token: 7일 만료, `JWT_REFRESH_SECRET`으로 서명
- 두 시크릿은 분리되어 있어 한쪽이 유출돼도 다른 토큰 타입은 위조 불가
