# ── 빌드 ────────────────────────────────────────────────────
FROM golang:1.27-alpine AS build

WORKDIR /src

# go.mod 만 먼저 복사한다. 소스가 바뀌어도 의존성 계층은 다시 받지 않는다.
# (이 프로젝트는 외부 의존성이 0개라 지금은 차이가 없지만, 늘어나면 효과가 있다)
COPY go.mod ./
RUN go mod download

COPY . .

# CGO 를 끄고 정적 링크한다. distroless 에는 libc 가 없다.
# -trimpath: 빌드한 사람의 경로가 바이너리에 남지 않게
# -s -w: 디버그 심볼 제거 (이미지 크기)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o /out/server ./cmd/server

# 제도 데이터가 유효하지 않으면 이미지를 만들지 않는다.
# ★ 깨진 제도를 배포한 뒤 발표장에서 알게 되면 늦는다.
RUN CGO_ENABLED=0 go run ./cmd/validate

# ── 실행 ────────────────────────────────────────────────────
# distroless: 셸도 패키지 관리자도 없다. 공격 표면이 최소가 된다.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=build /out/server /app/server
# 제도 JSON·기준중위소득 표·데모 캐시. 읽기 전용으로만 쓴다
COPY --from=build /src/data /app/data

# ★ root 로 돌지 않는다
USER nonroot:nonroot

EXPOSE 8080

# 사용자 입력을 디스크에 쓰지 않으므로 볼륨이 필요 없다.
ENTRYPOINT ["/app/server"]
