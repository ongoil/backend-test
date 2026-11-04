# User Management API

ระบบจัดการผู้ใช้ (User Management) แบบ RESTful และ gRPC เขียนด้วยภาษา Go
ใช้ [Fiber](https://gofiber.io) เป็น web framework, MongoDB เก็บข้อมูล และ JWT (HS256) ยืนยันตัวตน
โครงสร้างโปรเจกต์ออกแบบตามแนวทาง Hexagonal Architecture (Ports and Adapters)

## ฟีเจอร์

- สมัครสมาชิก และ login เพื่อรับ JWT (HMAC-SHA256)
- CRUD ผู้ใช้ที่ต้องใช้ JWT (สร้าง, ดูตาม ID, ดูทั้งหมด, แก้ชื่อ/อีเมล, ลบ)
- เก็บข้อมูลใน MongoDB ด้วย official Go driver (v2) และมี unique index ที่ email
- Logging middleware บันทึก method, path, status และเวลาที่ใช้ ทั้ง HTTP และ gRPC
- Goroutine เบื้องหลังที่ log จำนวนผู้ใช้ทั้งหมดทุก 10 วินาที
- ตรวจสอบข้อมูลนำเข้า (validation) พร้อมข้อความ error ราย field
- Graceful shutdown เมื่อได้รับ `SIGINT`/`SIGTERM` โดยใช้ `context.Context`
- gRPC server ที่มี `CreateUser` และ `GetUser` ป้องกันด้วย token ใน metadata
- รองรับ Docker และ docker-compose
- Unit test ด้วย package `testing` มาตรฐาน และ mock ที่เขียนเอง

## สารบัญ

- [เริ่มต้นด้วย Docker](#เริ่มต้นด้วย-docker)
- [รันบนเครื่อง](#รันบนเครื่อง)
- [การตั้งค่า](#การตั้งค่า)
- [คู่มือ JWT](#คู่มือ-jwt)
- [REST API](#rest-api)
- [gRPC API](#grpc-api)
- [การทดสอบ](#การทดสอบ)
- [โครงสร้างโปรเจกต์](#โครงสร้างโปรเจกต์)
- [การตัดสินใจด้านการออกแบบและสมมติฐาน](#การตัดสินใจด้านการออกแบบและสมมติฐาน)

## เริ่มต้นด้วย Docker

สิ่งที่ต้องมี: Docker ที่มี Compose v2

```bash
cp .env.example .env
# แก้ไฟล์ .env แล้วตั้ง JWT_SECRET เป็นค่าสุ่ม (อย่างน้อย 32 bytes) เช่น
#   openssl rand -base64 48

docker compose up --build -d
curl http://localhost:8080/healthz      # {"status":"ok"}
docker compose logs -f api              # ดู log ของ request และตัวนับจำนวนผู้ใช้
```

| Service | Address |
| --- | --- |
| REST API | `http://localhost:8080` |
| gRPC | `localhost:50051` |
| MongoDB | `mongodb://localhost:27017` |

หยุดการทำงานด้วย `docker compose down` (เพิ่ม `-v` ถ้าต้องการลบ volume ข้อมูลของ MongoDB ด้วย)

## รันบนเครื่อง

สิ่งที่ต้องมี: Go 1.26 ขึ้นไป และ MongoDB

```bash
# เปิดเฉพาะ MongoDB (หรือจะใช้ MongoDB ของตัวเองก็ได้)
docker compose up -d mongo

# Bash
export JWT_SECRET="$(openssl rand -base64 48)"
go run .

# PowerShell
$env:JWT_SECRET = "replace-with-a-random-secret-of-at-least-32-bytes"
go run .
```

มี `Makefile` ไว้เป็นทางลัด (`make run`, `make test`, `make cover`, `make proto`,
`make docker-up`) สำหรับเครื่องที่มีคำสั่ง `make`

## การตั้งค่า

ค่าทั้งหมดอ่านจาก environment variable

| ตัวแปร | ค่าเริ่มต้น | คำอธิบาย |
| --- | --- | --- |
| `JWT_SECRET` | **ต้องตั้ง** | key สำหรับ sign token แบบ HMAC ยาวอย่างน้อย 32 bytes |
| `JWT_TTL` | `24h` | อายุของ token |
| `JWT_ISSUER` | `backend-test` | ค่า claim `iss` ซึ่งจะถูกตรวจตอน verify |
| `MONGO_URI` | `mongodb://localhost:27017` | connection string ของ MongoDB |
| `MONGO_DB` | `user_management` | ชื่อ database |
| `HTTP_PORT` | `8080` | port ของ REST |
| `GRPC_PORT` | `50051` | port ของ gRPC |
| `BCRYPT_COST` | `12` | ค่า work factor ของ bcrypt (4–31) |
| `USER_COUNT_INTERVAL` | `10s` | ความถี่ในการ log จำนวนผู้ใช้ |
| `SHUTDOWN_TIMEOUT` | `15s` | เวลาสูงสุดที่รอให้ request ที่ค้างอยู่ทำงานเสร็จตอนปิดระบบ |
| `LOG_FORMAT` | `text` | `text` หรือ `json` |

ถ้าค่าตั้งค่าไม่ถูกต้อง server จะไม่ยอม start เช่น ไม่ได้ตั้ง `JWT_SECRET` หรือสั้นเกินไป

## คู่มือ JWT

### 1. สมัครสมาชิก

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Carol","email":"carol@example.com","password":"carolpass1"}'
```

### 2. Login เพื่อรับ token

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"carol@example.com","password":"carolpass1"}'
```

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJiYWNrZW5kLXRlc3QiLCJzdWIiOiI2YWU5OTIwNzNiNmExNTgzNjc3OWNiMGUiLCJleHAiOjE3OTM3Nzg1NjcsIm5iZiI6MTc5MzY5MjE2NywiaWF0IjoxNzkzNjkyMTY3fQ.KmTFseoHta62qFaOFk7pzvPt9zizWh-CruE-igJtO9I",
  "token_type": "Bearer",
  "expires_at": "2026-11-04T07:49:27Z"
}
```

### 3. เรียก endpoint ที่ต้องยืนยันตัวตน

ส่ง token ใน header `Authorization`

```bash
TOKEN="<access_token ที่ได้จากการ login>"
curl http://localhost:8080/api/v1/users -H "Authorization: Bearer $TOKEN"
```

สำหรับ gRPC ให้ส่งค่าเดียวกันใน metadata ชื่อ `authorization` (ดู [gRPC API](#grpc-api))

### รายละเอียดของ token

- Algorithm: **HS256** sign ด้วย `JWT_SECRET`
- Claims: `sub` (user ID), `iss`, `iat`, `nbf`, `exp`
- ตอน verify server จะตรวจ signature, **บังคับให้ algorithm เป็น HS256 เท่านั้น**
  (จึงกันการโจมตีด้วย `none` และ algorithm confusion ได้), ตรวจ issuer
  และบังคับว่าต้องมี `exp`
- token ที่ไม่มี, รูปแบบผิด, หมดอายุ หรือถูกแก้ไข จะได้ `401 Unauthorized`
  (gRPC: `Unauthenticated`)

payload ของ token ข้างบนเมื่อ decode แล้ว:

```json
{
  "iss": "backend-test",
  "sub": "6ae992073b6a15836779cb0e",
  "exp": 1793778567,
  "nbf": 1793692167,
  "iat": 1793692167
}
```

## REST API

Base URL: `http://localhost:8080`

| Method | Path | ต้องใช้ JWT | คำอธิบาย | สำเร็จ |
| --- | --- | --- | --- | --- |
| `GET` | `/healthz` | – | Health check | `200` |
| `POST` | `/api/v1/auth/register` | – | สมัครสมาชิก | `201` |
| `POST` | `/api/v1/auth/login` | – | Login และรับ JWT | `200` |
| `POST` | `/api/v1/users` | ✅ | สร้างผู้ใช้ | `201` |
| `GET` | `/api/v1/users` | ✅ | ดูผู้ใช้ทั้งหมด | `200` |
| `GET` | `/api/v1/users/{id}` | ✅ | ดูผู้ใช้ตาม ID | `200` |
| `PATCH` | `/api/v1/users/{id}` | ✅ | แก้ชื่อ และ/หรือ อีเมล | `200` |
| `DELETE` | `/api/v1/users/{id}` | ✅ | ลบผู้ใช้ | `204` |

### กฎการตรวจสอบข้อมูล

| Field | กฎ |
| --- | --- |
| `name` | ต้องมี ยาว 1–100 ตัวอักษร (ตัดช่องว่างหน้า/หลังออก) |
| `email` | ต้องมี รูปแบบอีเมลถูกต้อง ยาวไม่เกิน 254 ตัวอักษร เก็บเป็นตัวพิมพ์เล็ก |
| `password` | ต้องมี ยาว 8–72 ตัวอักษร (72 คือขีดจำกัดของ bcrypt) |

body ของ request ต้องเป็น JSON object เดียว ขนาดไม่เกิน 1 MiB และห้ามมี field ที่ไม่รู้จัก
เช่น ถ้า `PATCH` แล้วส่ง `"password"` มาจะได้ `400`

### รูปแบบ error

ทุก error ใช้รูปแบบเดียวกัน:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "request validation failed",
    "fields": {
      "email": "must be a valid email address",
      "name": "is required",
      "password": "must be at least 8 characters"
    }
  }
}
```

| HTTP status | `code` | เกิดเมื่อ |
| --- | --- | --- |
| `400` | `validation_failed` | ข้อมูลไม่ผ่าน validation (`fields` บอกปัญหาของแต่ละ field) |
| `400` | `invalid_json` | JSON ผิดรูปแบบ, body ว่าง, มี field ที่ไม่รู้จัก หรือชนิดข้อมูลผิด |
| `401` | `unauthorized` | ไม่มี token, รูปแบบผิด, ไม่ถูกต้อง หรือหมดอายุ |
| `401` | `invalid_credentials` | อีเมลหรือรหัสผ่านผิดตอน login |
| `404` | `not_found` | ไม่พบผู้ใช้ |
| `404` | `route_not_found` | ไม่มี path นี้ |
| `405` | `method_not_allowed` | path มีอยู่แต่ไม่รองรับ method นี้ |
| `413` | `payload_too_large` | body ใหญ่เกิน 1 MiB |
| `409` | `email_already_exists` | อีเมลนี้มีผู้ใช้อื่นใช้แล้ว |
| `500` | `internal_error` | เกิดข้อผิดพลาดที่ไม่คาดคิด (รายละเอียดจะถูก log ไว้ ไม่ส่งกลับให้ client) |

### ตัวอย่าง request และ response

ตัวอย่างทั้งหมดได้มาจากการรันจริงด้วย Docker

**สมัครสมาชิก**

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Carol","email":"carol@example.com","password":"carolpass1"}'
```

```http
HTTP/1.1 201 Created

{"id":"6ae992073b6a15836779cb0e","name":"Carol","email":"carol@example.com","created_at":"2026-11-03T07:49:27Z"}
```

**สมัครด้วยอีเมลที่มีอยู่แล้ว** (เปรียบเทียบแบบไม่สนตัวพิมพ์เล็ก/ใหญ่)

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Carol","email":"CAROL@example.com","password":"carolpass1"}'
```

```http
HTTP/1.1 409 Conflict

{"error":{"code":"email_already_exists","message":"email already exists"}}
```

**Login ด้วยรหัสผ่านผิด**

```http
HTTP/1.1 401 Unauthorized

{"error":{"code":"invalid_credentials","message":"invalid email or password"}}
```

**เรียกโดยไม่มี token**

```bash
curl http://localhost:8080/api/v1/users
```

```http
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Bearer realm="api"

{"error":{"code":"unauthorized","message":"missing or malformed Authorization header"}}
```

**สร้างผู้ใช้**

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Dave","email":"dave@example.com","password":"davepass1"}'
```

```http
HTTP/1.1 201 Created
Location: /api/v1/users/6ae992083b6a15836779cb10

{"id":"6ae992083b6a15836779cb10","name":"Dave","email":"dave@example.com","created_at":"2026-11-03T07:49:28.525Z"}
```

**ดูผู้ใช้ทั้งหมด**

```bash
curl http://localhost:8080/api/v1/users -H "Authorization: Bearer $TOKEN"
```

```http
HTTP/1.1 200 OK

{
  "users": [
    {"id":"6ae992073b6a15836779cb0e","name":"Carol","email":"carol@example.com","created_at":"2026-11-03T07:49:27Z"},
    {"id":"6ae992083b6a15836779cb10","name":"Dave","email":"dave@example.com","created_at":"2026-11-03T07:49:28.525Z"}
  ],
  "total": 2
}
```

**ดูผู้ใช้ตาม ID**

```bash
curl http://localhost:8080/api/v1/users/6ae992083b6a15836779cb10 -H "Authorization: Bearer $TOKEN"
```

```http
HTTP/1.1 200 OK

{"id":"6ae992083b6a15836779cb10","name":"Dave","email":"dave@example.com","created_at":"2026-11-03T07:49:28.525Z"}
```

**แก้ไขผู้ใช้** (แก้เฉพาะ field ที่ส่งมา)

```bash
curl -X PATCH http://localhost:8080/api/v1/users/6ae992083b6a15836779cb10 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"David"}'
```

```http
HTTP/1.1 200 OK

{"id":"6ae992083b6a15836779cb10","name":"David","email":"dave@example.com","created_at":"2026-11-03T07:49:28.525Z"}
```

**ลบผู้ใช้**

```bash
curl -X DELETE http://localhost:8080/api/v1/users/6ae992083b6a15836779cb10 -H "Authorization: Bearer $TOKEN"
```

```http
HTTP/1.1 204 No Content
```

**ดูผู้ใช้ที่ถูกลบไปแล้ว**

```http
HTTP/1.1 404 Not Found

{"error":{"code":"not_found","message":"user not found"}}
```

## gRPC API

นิยามอยู่ที่ [`proto/user/v1/user.proto`](proto/user/v1/user.proto)

```protobuf
service UserService {
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
}
```

ทุกการเรียกต้องส่ง metadata `authorization: Bearer <token>` โดยรับ token ได้จาก
endpoint login ของ REST ระบบเปิด server reflection ไว้ จึงใช้
[grpcurl](https://github.com/fullstorydev/grpcurl) ได้โดยไม่ต้องมีไฟล์ proto

```bash
grpcurl -plaintext localhost:50051 list

grpcurl -plaintext -H "authorization: Bearer $TOKEN" \
  -d '{"name":"Erin","email":"erin@example.com","password":"erinpass1"}' \
  localhost:50051 user.v1.UserService/CreateUser
```

```json
{
  "user": {
    "id": "6ae992193b6a15836779cb11",
    "name": "Erin",
    "email": "erin@example.com",
    "createdAt": "2026-11-03T07:49:45.893Z"
  }
}
```

```bash
grpcurl -plaintext -H "authorization: Bearer $TOKEN" \
  -d '{"id":"6ae992193b6a15836779cb11"}' \
  localhost:50051 user.v1.UserService/GetUser
```

| gRPC code | เกิดเมื่อ |
| --- | --- |
| `InvalidArgument` | ไม่ผ่าน validation โดยมี detail แบบ `google.rpc.BadRequest` บอกปัญหาแต่ละ field |
| `Unauthenticated` | ไม่มี token หรือ token ไม่ถูกต้อง |
| `NotFound` | ไม่พบผู้ใช้ |
| `AlreadyExists` | อีเมลนี้ถูกใช้แล้ว |
| `Internal` | เกิดข้อผิดพลาดที่ไม่คาดคิด (รายละเอียดอยู่ใน log เท่านั้น) |

ถ้าแก้ไฟล์ proto แล้วต้องการ generate โค้ด Go ใหม่ ให้ติดตั้ง `protoc`, `protoc-gen-go`
และ `protoc-gen-go-grpc` แล้วรัน `make proto` ไฟล์ที่ generate แล้ว commit ไว้ในโปรเจกต์
จึงไม่จำเป็นต้องทำขั้นตอนนี้เพื่อ build หรือรันโปรเจกต์

## การทดสอบ

```bash
go test ./...                                   # unit test
go test -cover ./...                            # พร้อม coverage
go test -race ./...                             # race detector (ต้องเปิด cgo)

# Integration test กับ MongoDB จริง
docker compose up -d mongo
MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./repository/mongodb/...
```

สิ่งที่ทดสอบ:

| Package | สิ่งที่เทสตรวจสอบ |
| --- | --- |
| `usecase` | business logic โดย mock repository: การ normalize ข้อมูล, validation, อีเมลซ้ำ, ไม่พบผู้ใช้, การแก้ไขบาง field, error จาก repository และการ hash, การ login (รหัสผ่านผิดกับอีเมลที่ไม่มีอยู่ได้ error เดียวกัน), goroutine นับผู้ใช้ทำงานตามรอบและหยุดได้ |
| `api` | HTTP ทั้งชุดผ่าน `app.Test` ของ Fiber (router, middleware, handler, usecase จริง, JWT จริง) บน repository ที่ mock ไว้: status code, error body, JWT ป้องกันทุก route, ไม่มีรหัสผ่านใน response, รายละเอียด internal error ไม่หลุดไปถึง client |
| `middleware` | middleware logging (รวม status ที่มาจาก error handler), recovery และ auth แยกแต่ละตัว |
| `grpcapi` | gRPC server จริงผ่านการเชื่อมต่อในหน่วยความจำ (`bufconn`): กรณีสำเร็จ, error code, รายละเอียด validation, auth interceptor |
| `security` | การ hash ด้วย bcrypt; การออกและ verify JWT รวมถึงการปฏิเสธ token ที่หมดอายุ, ถูกแก้ไข, secret ผิด, issuer ผิด, ใช้ `HS512`, ใช้ `none`, ไม่มี `exp` และไม่มี `sub` |
| `config` | ค่าเริ่มต้น, การ override, ค่าที่ไม่ถูกต้อง |
| `model` | ข้อความของ ValidationError |
| `repository/mongodb` | unit test: การแปลง `model.User` ↔ document และชื่อ field ใน bson; integration test (build tag `integration`) กับ MongoDB จริง: CRUD ของ repository, `Migrate` รันซ้ำได้ และ schema validator ปฏิเสธ document ที่ไม่ถูกต้อง |

การ mock MongoDB ใช้ `UserRepository` แบบ in-memory ที่เขียนเอง
([`mock/mock.go`](mock/mock.go)) ไม่ได้ใช้ library สำหรับ mock

## โครงสร้างโปรเจกต์

```
.
├── main.go                      # จุดเริ่มโปรแกรม: ประกอบ repository → usecase → handler, เรียก router, graceful shutdown
├── config/config.go             # การตั้งค่าจาก environment variable
├── router/router.go             # NewApp (fiber + middleware หลัก) และ SetRouter (route path ทั้งหมด)
├── api/                         # handler ของ REST
│   ├── auth.go                  # Register, Login
│   ├── user.go                  # Health, Create, List, Get, Update, Delete
│   ├── variable.go              # struct ของ request / response
│   └── user_test.go
├── middleware/middleware.go     # Logging, Recover, Authenticate (JWT)
├── dto/response.go              # รูปแบบ JSON response, แปลง error เป็น HTTP status
├── usecase/                     # business logic
│   ├── user.go                  # สร้าง/ดู/แก้/ลบ ผู้ใช้
│   ├── auth.go                  # สมัครสมาชิก, login
│   ├── user_counter.go          # goroutine log จำนวนผู้ใช้ทุก 10 วินาที
│   ├── validator.go             # validation ที่ใช้ร่วมกัน
│   └── variable.go              # struct input และ interface ที่ usecase ต้องใช้
├── repository/
│   ├── user.go                  # interface UserRepository
│   └── mongodb/
│       ├── model.go             # struct document ของ MongoDB (bson tags) + แปลงไป/กลับ model.User
│       └── user.go              # ตัวที่ใช้ MongoDB จริง
├── model/                       # domain: ไม่พึ่งพา MongoDB, Fiber หรือ gRPC
│   ├── model.go                 # error กลาง (ErrNotFound ฯลฯ) และ ValidationError
│   └── user.go                  # struct User (ไม่มี tag)
├── database/
│   ├── database.go              # Connect
│   └── table.go                 # Migrate: collection, schema validator, index
├── security/
│   ├── bcrypt.go                # hash รหัสผ่าน
│   └── jwt.go                   # ออกและ verify JWT
├── grpcapi/user.go              # gRPC server และ interceptor
├── proto/user/v1/               # นิยาม .proto และโค้ด gRPC ที่ generate แล้ว
├── logger/logger.go             # ตั้งค่า log
├── mock/mock.go                 # mock repository, hasher, token สำหรับเทส
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

การแบ่งชั้นตามแนวทาง Hexagonal (Ports and Adapters):

| ชั้น | Package | หน้าที่ |
| --- | --- | --- |
| Domain | `model` | entity และ error ไม่ import อะไรนอกจาก standard library |
| Application | `usecase`, `repository` (interface) | business logic และ interface (port) ที่ usecase ต้องการ เช่น `UserRepository`, `PasswordHasher`, `TokenIssuer` |
| Infrastructure (adapter) | `api`, `router`, `middleware`, `dto`, `grpcapi`, `repository/mongodb`, `database`, `security` | ส่วนที่รู้จัก Fiber, gRPC, MongoDB driver, bcrypt และ JWT |

```
router → api / grpcapi → usecase → repository (interface) ← repository/mongodb
                            │                                     │
                            └──────────────► model ◄──────────────┘
```

- **model** เป็น struct ล้วนไม่มี tag ทุกชั้นใช้ร่วมกันได้โดยไม่ผูกกับ MongoDB
- **repository/mongodb** มี struct document ของตัวเอง (bson tag) ใน `model.go`
  และแปลงไป/กลับ `model.User` เอง ถ้าเปลี่ยน database ก็แก้แค่ package นี้
- **usecase** พึ่งพาแค่ `model` และ interface จึงเทสด้วย mock ได้โดยไม่ต้องมี MongoDB
- **api** และ **grpcapi** แปลง request เป็นการเรียก usecase แล้วแปลงผลลัพธ์กลับเป็น response
  ผ่าน struct ของตัวเอง (`api/variable.go`, proto) จึงไม่ต้องมี json tag ใน model
- **main.go** เป็นที่เดียวที่นำทุกส่วนมาประกอบกัน

## การตัดสินใจด้านการออกแบบและสมมติฐาน

**Library ที่ใช้**

| หน้าที่ | Library | เหตุผล |
| --- | --- | --- |
| Web framework | `gofiber/fiber/v3` | เร็ว (สร้างบน fasthttp) API ใช้ง่าย มี route group และ middleware ในตัว |
| JWT | `golang-jwt/jwt/v5` | library JWT มาตรฐานของ Go กำหนด algorithm ที่ยอมรับได้ |
| Validation | `go-playground/validator/v10` | กำหนดกฎด้วย struct tag ใช้กันแพร่หลาย |
| MongoDB | `go.mongodb.org/mongo-driver/v2` | driver ทางการ (โจทย์กำหนด) |
| รหัสผ่าน | `golang.org/x/crypto/bcrypt` | มาตรฐานสำหรับ hash รหัสผ่าน |
| gRPC | `google.golang.org/grpc` | implementation ทางการ |
| Logging | `log/slog` | structured logging ใน standard library |

**การยืนยันตัวตนและความปลอดภัย**

- รหัสผ่าน hash ด้วย bcrypt และไม่มี endpoint ไหนส่งรหัสผ่านกลับไป
- ตอน login อีเมลที่ไม่มีอยู่กับรหัสผ่านผิดจะได้ error เดียวกัน และถ้าไม่พบอีเมล
  ระบบจะเทียบกับ hash หลอกแทน เพื่อไม่ให้เวลาตอบสนองเผยว่ามีบัญชีนั้นอยู่หรือไม่
- รับเฉพาะ token แบบ HS256 และ secret ต้องยาวอย่างน้อย 32 bytes
  ถ้า secret อ่อนหรือไม่ได้ตั้ง server จะไม่ start
- **สิทธิ์การใช้งาน:** ผู้ใช้ที่ login แล้วทุกคนจัดการผู้ใช้คนอื่นได้ทั้งหมด
  เพราะโจทย์ไม่ได้กล่าวถึง role จึงไม่มีการตรวจสิทธิ์ admin/เจ้าของ
  ถ้าใช้งานจริงควรเพิ่ม role หรือจำกัดให้แก้ไข/ลบได้เฉพาะข้อมูลของตัวเอง
- Token เป็นแบบ stateless จึงยังใช้ได้จนหมดอายุแม้ผู้ใช้จะถูกลบไปแล้ว
  ถ้าต้องการเพิกถอน token ต้องใช้ denylist หรือ access token อายุสั้นคู่กับ refresh token
- `POST /auth/register` เปิดให้ใครก็ได้สมัคร ส่วน `POST /users` ทำงานเหมือนกันแต่ต้องมี token
  (ผู้ใช้ที่ login แล้วสร้างบัญชีให้คนอื่น)
- gRPC ใช้ plaintext สำหรับการพัฒนาบนเครื่อง ถ้าใช้งานจริงควรใช้ TLS

**ข้อมูล**

- แยก model ของ domain ออกจาก database: `model.User` ใน [`model/user.go`](model/user.go)
  ไม่มี tag และ `ID` เป็น `string` ส่วนรูปแบบที่เก็บใน MongoDB (`ObjectID`, bson tag)
  อยู่ใน [`repository/mongodb/model.go`](repository/mongodb/model.go)
  แลกกับการต้องแปลง struct ใน repository แต่ทำให้ domain ไม่ผูกกับ MongoDB ตามที่โจทย์ข้อ Hexagonal ต้องการ
- รหัสผ่านไม่หลุดออกไปใน response เพราะ api ส่งกลับผ่าน `responseUser` ที่ไม่มี field รหัสผ่านเลย
- **Migration:** `database.Migrate` ใน [`database/table.go`](database/table.go)
  ถูกเรียกทุกครั้งที่ server start ทำหน้าที่สร้าง collection `users` พร้อม `$jsonSchema`
  validator (บังคับ field ที่ต้องมี ชนิดข้อมูล และความยาว) และ unique index ที่ `email`
  ถ้า collection มีอยู่แล้วจะอัปเดต validator ด้วย `collMod` จึงรันซ้ำได้ปลอดภัย (idempotent)
  การเพิ่ม collection หรือ index ใหม่ทำได้โดยเพิ่ม spec ในตัวแปร `collections`
- อีเมลถูกตัดช่องว่างและเก็บเป็นตัวพิมพ์เล็ก และ MongoDB มี unique index ที่ `email`
  ดังนั้น `Alice@x.com` กับ `alice@x.com` ถือเป็นอีเมลเดียวกัน
  ระบบใช้ index เป็นตัวตัดสิน (ไม่ใช่การอ่านก่อนเขียน) จึงป้องกันการสร้างซ้ำจาก request ที่มาพร้อมกันได้
- ID คือ MongoDB ObjectID ส่งกลับเป็นสตริง hex ถ้า ID รูปแบบผิดจะได้ `404`
  (เพราะไม่มีทางตรงกับผู้ใช้คนไหน) แทนที่จะเป็น `400`
- `created_at` เก็บเป็นเวลา UTC ความละเอียดระดับมิลลิวินาที (ตามความละเอียดของ MongoDB)
- `PATCH` แก้ได้เฉพาะ `name` และ/หรือ `email` และต้องส่งมาอย่างน้อยหนึ่ง field
  การเปลี่ยนรหัสผ่านอยู่นอกขอบเขตของโจทย์
- `GET /users` คืนผู้ใช้ทั้งหมดโดยไม่แบ่งหน้าตามที่โจทย์กำหนด
  ถ้าข้อมูลมีจำนวนมากควรเพิ่มการแบ่งหน้าแบบ `limit`/cursor

**การทำงานของระบบ**

- Goroutine นับผู้ใช้จะ log ทุก `USER_COUNT_INTERVAL` (ค่าเริ่มต้น 10 วินาที)
  ถ้านับไม่สำเร็จจะ log error แล้วทำงานต่อ
- เมื่อได้รับ `SIGINT`/`SIGTERM` server จะหยุดรับ request ใหม่ รอให้ HTTP และ gRPC
  ทำ request ที่ค้างอยู่ให้เสร็จ (ไม่เกิน `SHUTDOWN_TIMEOUT`) หยุด goroutine เบื้องหลัง
  แล้วตัดการเชื่อมต่อ MongoDB เป็นลำดับสุดท้าย
- Docker image เป็น static binary บน `distroless/static` รันด้วย user ที่ไม่ใช่ root
  และ Compose จะรอให้ health check ของ MongoDB ผ่านก่อนจึงเริ่ม API
