---
name: go-layered-structure
description: Apply this repo's Go clean-architecture layout conventions (dto request/response split, internal/config, per-concern middleware files, internal/auth package) when adding new features or restructuring code.
---

# Go Layered Structure

이 프로젝트(clone-youtube-backend)가 따르는 디렉토리 컨벤션. `Dev-HyunSang/my-own-library-backend`를 참고해 정한 구조이며, 새 기능을 추가하거나 구조를 정리할 때 이 규칙을 따른다.

## 레이어

```
internal/
├── auth/            # JWT 발급/검증, 비밀번호 해싱 등 인증 관련 순수 로직
├── config/           # 환경변수 로딩 (기존 최상위 config/ 대신 여기)
├── db/                # DB 커넥션
├── domain/            # 순수 도메인 모델 (외부 의존 없음)
├── dto/
│   ├── request/        # 요청 바디 바인딩용 구조체
│   └── response/        # 응답 바디용 구조체
├── handler/           # gin 핸들러 (usecase 호출만, 비즈니스 로직 없음)
├── middleware/
│   ├── auth.go          # 인증 미들웨어
│   ├── admin.go          # 관리자 권한 체크 미들웨어
│   └── request_id.go     # 요청 ID 부여 미들웨어
├── repository/
│   ├── postgressql/       # PostgreSQL 구현체
│   └── redis/             # Redis 구현체
├── service/           # 외부 연동 (이메일 등)
└── usecase/           # 비즈니스 로직, handler와 repository 사이
```

## 규칙

- **핸들러는 얇게**: `internal/handler/*.go`는 요청 파싱 → usecase 호출 → 응답 매핑만 한다. 비즈니스 로직은 usecase로.
- **DTO는 방향으로 분리**: 요청 바인딩 struct는 `internal/dto/request/`, 응답 struct는 `internal/dto/response/`. 도메인 모델(`internal/domain`)을 API 응답에 직접 노출하지 않는다.
- **설정은 `internal/config`에만**: 최상위에 별도 `config/` 패키지를 새로 만들지 않는다.
- **미들웨어는 관심사별 파일로**: 새 미들웨어를 추가할 때 기존 파일에 이어붙이지 말고 목적에 맞는 파일(`auth.go`/`admin.go`/`request_id.go`, 없으면 새 파일)에 작성한다.
- **인증 로직은 `internal/auth`에, 미들웨어는 그걸 호출만**: JWT 발급/파싱, 비밀번호 해싱 같은 실제 로직은 `internal/auth`에 두고, `internal/middleware/auth.go`는 그 함수를 호출해 gin 컨텍스트에 적용하는 역할만 한다.
- **repository는 구현체 이름으로 하위 폴더**: 새 저장소를 추가하면 `internal/repository/<impl>/` 형태를 유지한다 (예: 캐시 계층이 필요하면 `internal/cache/`처럼 repository와 분리할지, `internal/repository/redis/`에 넣을지는 그 시점에 판단).

## 스캐폴딩 지침

- 아직 구현할 로직이 없는 레이어/파일은 빈 채로 두거나 `// ponytail: ...` 주석으로 "왜 비어있는지 + 언제 채울지"만 남긴다. 가짜 인터페이스나 목업 구현을 미리 만들지 않는다.
- book/fcm/scheduler 등 참고 저장소(my-own-library-backend)에만 있고 이 프로젝트 도메인(유튜브 클론)과 무관한 기능은 가져오지 않는다.
