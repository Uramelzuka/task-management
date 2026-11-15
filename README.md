# Task Management

Fullstack task manager: Go (Gin) + MySQL 8.4 + Redis 7 backend, React Native CLI (0.88, tanpa Expo) + TypeScript mobile app.

```
.
├── docker-compose.yml        mysql (3307) · redis (6379) · migrate · api (8080)
├── backend/
│   ├── cmd/api/              entrypoint, wiring, graceful shutdown
│   ├── internal/domain/      Task, ListParams, domain errors
│   ├── internal/handler/     HTTP (Gin), error envelope, routing  ← tests here
│   ├── internal/service/     validation, query normalisation, cache orchestration
│   ├── internal/repository/  MySQL (interface + impl)
│   ├── internal/cache/       Redis (interface + impl)
│   └── migrations/           golang-migrate SQL
└── mobile/                   React Native CLI
    ├── App.tsx
    ├── android/ ios/         native project
    └── src/{api,hooks,components,screens,config.ts}
```

## Menjalankan

### Backend (semua lewat Docker)

```bash
docker compose up --build
```

Urutannya: `mysql` dan `redis` healthy → `migrate up` jalan sekali → `api` start di `http://localhost:8080`.

Cek cepat:

```bash
curl -s localhost:8080/healthz
curl -s -X POST localhost:8080/api/tasks -H 'Content-Type: application/json' -d '{"title":"First task","assignee":"alice"}'
curl -s 'localhost:8080/api/tasks?keyword=first&status=todo&page=1&limit=10&sort=-created_at'
```

### Backend lokal (tanpa container api)

```bash
docker compose up -d mysql redis migrate
cd backend && go run ./cmd/api
```

Env (default di dalam kurung):
`PORT` (8080), `DB_DSN` (`task:task@tcp(localhost:3307)/taskdb?parseTime=true&clientFoundRows=true&loc=UTC`), `REDIS_ADDR` (localhost:6379), `CACHE_TTL` (60s).

### Mobile

Prasyarat Android: JDK 17, Android SDK, dan `ANDROID_HOME` mengarah ke folder SDK dengan `platform-tools` ada di `PATH` (lihat [panduan environment RN](https://reactnative.dev/docs/set-up-your-environment)). Emulator harus sudah menyala atau HP tersambung (`adb devices`). Build native memakai CMake 3.31.6 (dipasang di `android/app/build.gradle`) karena ninja bawaan CMake 3.22.1 gagal untuk path Windows lebih dari 260 karakter. Instal sekali lewat `sdkmanager "cmake;3.31.6"` atau Android Studio → SDK Manager → SDK Tools → CMake.

```bash
cd mobile
npm install
npm start               # terminal 1: Metro bundler
npm run android         # terminal 2: build + install ke emulator/HP
```

iOS (macOS saja): `cd ios && bundle install && bundle exec pod install && cd .. && npm run ios`.

Base URL API diatur di `mobile/src/config.ts`: Android emulator otomatis memakai `http://10.0.2.2:8080`, iOS simulator memakai `http://localhost:8080`. Untuk HP fisik, isi `API_URL_OVERRIDE` dengan `http://<IP-LAN-laptop>:8080`. HTTP biasa (cleartext) hanya diizinkan di build debug; build release butuh HTTPS.

### Test

```bash
cd backend && go test ./...
cd mobile && npm test && npm run typecheck
```

Test backend tidak butuh Docker: memakai `httptest` dengan router production, Redis diganti `miniredis`, MySQL diganti repository in-memory yang mengimplementasikan interface `TaskRepository`.

## API

| Method | Path | Sukses |
|---|---|---|
| GET | `/api/tasks?status=&keyword=&assignee=&page=&limit=&sort=` | 200 `{data:[...], meta:{page,limit,total,total_pages}}` |
| GET | `/api/tasks/{id}` | 200 `{data:{...}}` |
| POST | `/api/tasks` | 201 `{data:{...}}` |
| PUT | `/api/tasks/{id}` | 200 `{data:{...}}` |
| DELETE | `/api/tasks/{id}` | 204 (soft delete) |

- `status`: `todo` \| `in_progress` \| `done`
- `keyword`: dicari di `title` dan `description` (case-insensitive, `%`/`_` diperlakukan literal)
- `assignee`: exact match
- `page` ≥ 1 (default 1), `limit` 1–100 (default 10)
- `sort`: `id`, `title`, `status`, `assignee`, `due_date`, `created_at`, `updated_at`; prefix `-` untuk descending. Default `-created_at`.

Body PUT/POST: `{"title": "...", "description": "...", "status": "todo", "assignee": "...", "due_date": "YYYY-MM-DD"}`. Hanya `title` yang wajib. PUT adalah full replace: field opsional yang tidak dikirim akan menjadi `null`.

Semua error memakai format yang sama:

```json
{"error": {"code": "DUPLICATE_TITLE", "message": "a task with this title already exists"}}
```

| HTTP | code |
|---|---|
| 400 | `VALIDATION_ERROR`, `INVALID_JSON` |
| 404 | `NOT_FOUND` (termasuk task yang sudah soft-deleted dan route yang tidak ada) |
| 405 | `METHOD_NOT_ALLOWED` |
| 409 | `DUPLICATE_TITLE` |
| 500 | `INTERNAL_ERROR` (detail tidak pernah dibocorkan ke client, hanya di-log) |

## Keputusan desain

**Layering `handler → service → repository/cache`.** Handler hanya mengurus HTTP (parsing, status code). Service menangani validasi, normalisasi query, dan orkestrasi cache. Repository dan cache berupa interface, jadi test bisa memakai fake tanpa mengubah kode production. Mapping error domain ke HTTP ada di satu tempat (`respondError`).

**Unique title yang mengabaikan soft-deleted.** `active_title` adalah generated column `IF(deleted_at IS NULL, title, NULL)` dengan `UNIQUE` index. Row yang di-soft-delete bernilai `NULL`, dan unique index InnoDB mengizinkan banyak `NULL`, sehingga judul bisa dipakai lagi setelah task lama dihapus. Keunikan ditegakkan di database, bukan dengan cek-lalu-insert di aplikasi yang rawan race condition. MySQL error 1062 dipetakan ke `ErrDuplicateTitle` → 409. Collation `utf8mb4_0900_ai_ci` membuat "Alpha" dan "alpha" dianggap duplikat.

**Soft delete.** Setiap query baca dan tulis memakai `deleted_at IS NULL`, jadi task yang terhapus tidak muncul di list, `GET` by id, `PUT`, maupun `DELETE` ulang (semuanya 404).

**`clientFoundRows=true` di DSN.** Tanpa ini, MySQL melaporkan 0 affected rows untuk `UPDATE` yang nilainya sama persis dengan data lama, sehingga PUT idempotent akan salah dibaca sebagai 404. Ada test untuk kasus ini.

**Sort whitelist.** Nilai `sort` dari user tidak pernah masuk ke SQL. Nilai itu dipakai sebagai key ke `domain.SortField` (map ke nama kolom yang dipercaya), dan dicek di service maupun di repository. Selain itu `id` selalu ditambahkan sebagai tie-breaker supaya paging stabil.

**Cache list (Redis, TTL 60 detik).**
- Key: `tasks:list:v{version}:{params ternormalisasi}`. Normalisasi meliputi trim, lowercase keyword/status, kompres spasi, isi default page/limit/sort, dan urutan key yang stabil (`url.Values.Encode`). Jadi `?status=TODO&keyword= Foo ` dan `?keyword=foo&status=todo&page=1&limit=10&sort=-created_at` memakai satu key yang sama. Lowercase keyword aman karena pencarian di MySQL memang case-insensitive.
- Invalidasi: setiap create/update/delete yang **berhasil** menjalankan `INCR tasks:version`. Key versi lama tidak dihapus, hanya tidak pernah dibaca lagi dan akan hilang sendiri oleh TTL. Ini O(1), tanpa `SCAN`/`KEYS`.
- Race: service membaca version **sekali** lalu memakai key yang sama untuk GET dan SET. Kalau ada write di antara baca DB dan SET, hasil lama tersimpan di versi lama yang sudah tidak dibaca siapa pun, jadi data basi tidak pernah tersaji.
- Redis mati tidak membuat API mati: semua error cache di-log lalu request jatuh ke MySQL (ada test-nya).

**Mobile.**
- `useDebounce` (400 ms) memisahkan input search yang tetap responsif dari request. Ganti keyword/status selalu reset ke page 1.
- `useTasks` membatalkan request lama dengan `AbortController` setiap kali query berubah, supaya hasil keyword lama tidak menimpa hasil keyword baru.
- Bug "list tidak refresh setelah update": setelah save di modal berhasil, layar memanggil `reload()` (refetch), bukan menambal state lokal. Ini menjamin urutan sort, filter, dan total ikut benar, dan karena version cache sudah naik, hasilnya pasti segar. Ada regression test untuk ini.
- Error 409 dan validasi tampil inline di modal, dan modal tetap terbuka supaya isian tidak hilang.
- Loading state: spinner penuh saat load pertama, spinner kecil saat refetch/search sambil tetap menampilkan data lama (tidak berkedip), loading di tombol Save, dan pull-to-refresh.
- React Native CLI tanpa Expo; satu-satunya dependency native tambahan adalah `react-native-safe-area-context`. Hanya satu layar, jadi tidak memakai library navigasi.

## Trade-off

- **Write → cache bisa basi maksimal 60 detik kalau `INCR` gagal** (Redis down tepat setelah commit). Write tetap dianggap sukses karena data sudah tersimpan di DB. Alternatif yang lebih ketat (gagalkan request) tidak masuk akal setelah commit.
- **Version global, bukan per-filter.** Satu write menginvalidasi semua list. Sederhana dan selalu benar. Untuk beban write tinggi, hit rate akan turun. Optimisasinya (version per status/assignee) tidak sepadan untuk skala ini.
- **`LIKE '%kw%'` tidak memakai index**, jadi full scan pada data live. Cukup untuk ribuan row. Untuk data besar, pakai `FULLTEXT` index atau search engine terpisah.
- **Offset pagination**: sederhana dan cocok dengan UI "halaman X dari Y", tapi lambat di offset besar dan bisa bergeser kalau ada insert di antara halaman. Cursor pagination lebih tepat untuk infinite scroll.
- **`COUNT(*)` setiap request list.** Diperlukan untuk `total_pages` dan di-cache bersama halaman.
- **PUT = full replace** (bukan PATCH): kontrak jelas dan cocok dengan edit modal yang selalu mengirim semua field. Konsekuensinya client harus mengirim ulang field opsional.
- **Test backend memakai fake repository**, bukan MySQL asli. Jadi perilaku SQL (generated column, error 1062, `clientFoundRows`, escape `LIKE`) tidak tercakup oleh `go test`. Semuanya sudah diverifikasi lewat smoke test curl terhadap stack docker-compose (MySQL 8.4 + Redis 7). Langkah berikutnya: jadikan otomatis dengan integration test testcontainers.
- **Tidak ada auth**, sesuai scope assessment.
- Query string dengan `;` mentah ditolak seluruhnya oleh `net/url` Go, sehingga semua filter diabaikan (bukan 400). Ini perilaku standar Go dan bukan celah keamanan, karena `sort` tetap divalidasi whitelist.
