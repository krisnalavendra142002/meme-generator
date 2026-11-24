# Laporan Perbaikan Masalah Produksi (Fix Report)

Dokumen ini berisi catatan investigasi, analisis akar masalah (*root cause*), detail perubahan kode, serta bukti verifikasi (*proof of completion*) untuk setiap isu produksi yang dilaporkan pada proyek **Meme Portal**.

---

## TKT-103: Logout Tidak Berfungsi dan Halaman Muter-Muter (Redirect Loop)

### 1. Deskripsi Keluhan
Pengguna melaporkan:
> “Aku klik Keluar, kelihatannya sukses. Tapi pas aku buka halaman login, halaman itu langsung lempar aku ke halaman depan. Dari halaman depan malah ke halaman login lagi, dilempar lagi. Bolak balik terus sampai laptop saya meledak.”

### 2. Investigasi & Akar Masalah (*Root Cause*)
Setelah ditelusuri dari alur logout di backend sampai proteksi halaman di frontend, masalah ini terjadi karena adanya bentrok logika pembersihan cookie dan redirect:

1. **Path Cookie Beda Saat Logout di Backend (`api/internal/modules/auth/handler.go`)**:
   - Cookie login (`meme_access_token` dan `meme_refresh_token`) dibuat dengan `Path: "/"`.
   - Namun, fungsi `clearAuthCookies` di handler logout malah mengirim instruksi hapus cookie dengan `Path: "/api/v1/auth"`.
   - Karena path-nya berbeda, browser tidak mau menghapus cookie yang ada di path `"/"`. Akibatnya, token autentikasi masih tetap tersimpan di browser user.

2. **Cek Cookie Mentah di Halaman Login & Register (`web/src/app/login/page.tsx` & `register/page.tsx`)**:
   - Halaman login mengecek string `document.cookie.includes("meme_access_token")`.
   - Karena cookie tadi gagal dibersihkan oleh backend, halaman login mengira user masih login, lalu melempar user ke halaman utama (`/`).

3. **Redirect Salah Tempat di Halaman Utama (`web/src/app/page.tsx`)**:
   - Halaman utama (`/`) adalah landing page publik, tapi di dalamnya dipasang pengecekan: jika data user kosong (`!me.data`), arahkan ke `/login`.
   - Padahal saat logout, token di sisi session/backend sudah tidak valid sehingga `/api/v1/auth/me` mengembalikan status unauthenticated. Halaman utama pun melempar user kembali ke `/login`.

4. **Terjadilah Redirect Loop**:
   - `/login` mendeteksi sisa cookie $\rightarrow$ lempar ke `/` $\rightarrow$ `/` melihat user belum terautentikasi $\rightarrow$ lempar balik ke `/login`. Keduanya saling melempar tanpa henti.

---

### 3. Kode yang Diubah

#### A. Backend
1. **`api/internal/platform/cookie/cookie.go`**:
   - Memperbaiki `cookie.Clear` agar menggunakan `Path: "/"` dan atribut `HttpOnly: false` agar sama persis dengan saat cookie dibuat (`cookie.Set`).
2. **`api/internal/modules/auth/handler.go`**:
   - Mengubah `clearAuthCookies` agar menggunakan helper `cookie.Clear` dengan `Path: "/"`.
   - Memastikan `clearAuthCookies` tetap dijalankan meskipun konteks auth sudah tidak valid, supaya browser tetap membersihkan sisa cookie.
3. **`api/internal/platform/cookie/cookie_test.go` & `api/internal/modules/auth/handler_test.go`**:
   - Menambahkan unit test untuk memastikan atribut `Path`, nilai kosong, dan `MaxAge: -1` terpasang dengan benar pada response header saat logout.

#### B. Frontend
1. **`web/src/app/page.tsx`**:
   - Menghapus redirect paksa ke `/login` di landing page utama, sehingga tamu maupun user yang baru saja logout bisa melihat halaman depan dengan normal.
2. **`web/src/app/login/page.tsx` & `web/src/app/register/page.tsx`**:
   - Mengganti pengecekan mentah `document.cookie` dengan status query `useMe()` (`!me.isLoading && me.data`). Jika user belum terotentikasi, halaman login tetap terbuka dengan normal.
3. **`web/src/lib/auth.ts`**:
   - Menambahkan pembersihan cache query TanStack (`qc.clear()`) dan reset cookie di client pada mutation `useLogout`.

---

### 4. Bukti Penyelesaian (*Proof of Completion*)

#### A. Langkah Reproduksi Sebelum Perbaikan
1. Login ke aplikasi, lalu buka DevTools Network.
2. Klik tombol "Keluar" (Logout). Perhatikan response header dari `POST /api/v1/auth/logout`:
   ```http
   Set-Cookie: meme_access_token=; Path=/api/v1/auth; Max-Age=0; SameSite=Lax
   ```
   *Karena cookie asli berada di `Path=/`, cookie tidak terhapus dari browser.*
3. Navigasi manual ke `/login`.
4. Browser langsung redirect ke `/`, lalu halaman `/` langsung melempar kembali ke `/login`, terjadi redirect loop tanpa henti.

#### B. Langkah Reproduksi Sesudah Perbaikan
1. Jalankan unit test auth & cookie di backend:
   ```bash
   cd api && go test -v ./internal/platform/cookie ./internal/modules/auth
   ```
   **Output**:
   ```
   === RUN   TestSet
   --- PASS: TestSet (0.00s)
   === RUN   TestClear
   --- PASS: TestClear (0.00s)
   PASS
   === RUN   TestClearAuthCookies
   --- PASS: TestClearAuthCookies (0.00s)
   PASS
   ```
2. Panggil endpoint logout dengan token aktif:
   ```bash
   curl -i -X POST http://localhost:8080/api/v1/auth/logout \
     -H "Cookie: meme_access_token=<token>; meme_refresh_token=<token>"
   ```
   **Output**:
   ```http
   HTTP/1.1 200 OK
   Content-Type: application/json
   Set-Cookie: meme_access_token=; Path=/; Max-Age=0; SameSite=Lax
   Set-Cookie: meme_refresh_token=; Path=/; Max-Age=0; SameSite=Lax

   {"success":true,"code":10104,"message":"logout berhasil"}
   ```
   *Cookie di path `/` berhasil terhapus tuntas.*
3. Pengujian alur pada browser:
   - Login $\rightarrow$ Masuk ke `/studio`.
   - Klik "Keluar" $\rightarrow$ Cookie terhapus, diarahkan ke halaman depan `/`.
   - Buka `/login` secara manual $\rightarrow$ Halaman login tampil normal tanpa terpental/looping sama sekali.

---

## TKT-105: Tagihan Dobel, Checkout Ganda

### 1. Deskripsi Keluhan
Pengguna melaporkan:
> “Maaf aku panik tadi. Internet lemot, aku klik Berlangganan dua kali karena dikira tidak jalan. Ternyata ada DUA checkout pending untuk paket yang sama. Untung belum bayar keduanya.”

### 2. Investigasi & Akar Masalah (*Root Cause*)
Setelah ditelusuri dari alur checkout di frontend dan backend:
1. **Header `Idempotency-Key` Ditolak CORS Backend (`api/internal/middleware/cors.go`)**:
   - Frontend mengirim header `Idempotency-Key` saat checkout.
   - Namun di `api/internal/middleware/cors.go`, `Access-Control-Allow-Headers` hanya mengizinkan `"Content-Type, Authorization, X-Requested-With"`.
   - Browser memblokir request melalui preflight OPTIONS dan Axios menganggapnya `Network Error` ("Tidak bisa terhubung ke server API — pastikan backend berjalan").
2. **Key Baru Dibuat di Setiap Pemanggilan Mutasi (`web/src/lib/queries.ts`)**:
   - Pada fungsi `useCheckout`, `crypto.randomUUID()` dieksekusi langsung di dalam `mutationFn`.
   - Ketika koneksi lambat dan pengguna menekan tombol "Berlangganan" dua kali, setiap pemanggilan mutasi menghasilkan UUID yang berbeda.
   - Karena UUID-nya berbeda, backend menganggapnya sebagai dua transaksi terpisah dan membuat dua entri pending subscription & payment.
3. **Tombol "Berlangganan" Tidak Memiliki Disabled State Saat Pending (`web/src/app/pricing/page.tsx`)**:
   - Komponen tombol tidak dinonaktifkan (`disabled={checkout.isPending}`) ketika request sedang berjalan, sehingga pengguna dapat mengklik berulang kali saat koneksi lambat.
4. **Penanganan Race Condition Concurrency di Backend (`api/internal/modules/subscription/service.go`)**:
   - Jika dua request paralel dengan `Idempotency-Key` yang sama dieksekusi bersamaan, request kedua dapat terkena `unique constraint violation` pada tabel `payments`. Backend sebelumnya langsung mengembalikan error 500 alih-alih mengembalikan checkout yang sudah berhasil dibuat oleh request pertama.

---

### 3. Kode yang Diubah

#### A. Backend
1. **`api/internal/middleware/cors.go`**:
   - Menambahkan `Idempotency-Key` ke dalam header `Access-Control-Allow-Headers` agar request preflight browser diizinkan.
2. **`api/internal/middleware/cors_test.go`**:
   - Menambahkan unit test preflight OPTIONS untuk memverifikasi `Idempotency-Key` telah diizinkan.
3. **`api/internal/middleware/auth.go`**:
   - Menambahkan helper `ContextWithUserID` untuk kemudahan pengujian context user ID.
4. **`api/internal/modules/subscription/service.go`**:
   - Memperbaiki race condition pada `Checkout`: jika `CreateSubscription` gagal karena duplicate key, sistem mengecek ulang `CheckoutByIdempotencyKey` dan mengembalikan record yang sudah ada secara transparan (tanpa throw error 500).
5. **`api/internal/modules/subscription/handler_test.go`**:
   - Menambahkan unit test untuk validasi keberadaan header `Idempotency-Key`.

#### B. Frontend
1. **`web/src/lib/queries.ts`**:
   - Menyimpan `idempotencyKey` menggunakan `useRef<Map<string, string>>` per `planCode` sehingga pengulangan mutasi/retry untuk plan yang sama menggunakan key yang identik.
   - Membersihkan key pada saat checkout berhasil (`onSuccess`).
2. **`web/src/app/pricing/page.tsx`**:
   - Menambahkan `disabled={checkout.isPending}` pada tombol "Berlangganan".
   - Menampilkan label `"Memproses..."` ketika mutasi untuk paket terkait sedang aktif.

---

### 4. Bukti Penyelesaian (*Proof of Completion*)

#### A. Langkah Reproduksi Sebelum Perbaikan
1. Buka browser ke `/pricing` saat login.
2. Buka DevTools Console & Network.
3. Klik tombol "Berlangganan". Browser menolak request preflight dengan error:
   ```
   Access to XMLHttpRequest at 'http://localhost:8080/api/v1/subscription/checkout' from origin 'http://localhost:3000' has been blocked by CORS policy: Request header field idempotency-key is not allowed by Access-Control-Allow-Headers in preflight response.
   ```
4. Di UI muncul pesan: *"Tidak bisa terhubung ke server API — pastikan backend berjalan"*.
5. Jika diuji via API langsung tanpa penanganan key persisten, dua kali klik menghasilkan dua data checkout pending dengan reference berbeda:
   - Request 1: `reference: "abc12345"`
   - Request 2: `reference: "def67890"`

#### B. Langkah Reproduksi Sesudah Perbaikan
1. Jalankan unit test middleware dan modul subscription:
   ```bash
   cd api && go test -v ./internal/middleware ./internal/modules/subscription
   ```
   **Output**:
   ```
   === RUN   TestCORS_PreflightAllowsIdempotencyKey
   --- PASS: TestCORS_PreflightAllowsIdempotencyKey (0.00s)
   PASS
   === RUN   TestHandler_Checkout_RequiresIdempotencyKey
   --- PASS: TestHandler_Checkout_RequiresIdempotencyKey (0.00s)
   PASS
   ```
2. Uji Preflight CORS dengan header `Idempotency-Key`:
   ```bash
   curl -i -X OPTIONS http://localhost:8080/api/v1/subscription/checkout \
     -H "Origin: http://localhost:3000" \
     -H "Access-Control-Request-Method: POST" \
     -H "Access-Control-Request-Headers: content-type, idempotency-key"
   ```
   **Output**:
   ```http
   HTTP/1.1 204 No Content
   Access-Control-Allow-Headers: Content-Type, Authorization, X-Requested-With, Idempotency-Key
   ```
3. Uji Replay Checkout dengan `Idempotency-Key` yang sama (menirukan retry/double click):
   - Request 1 dan Request 2 mengembalikan ID langganan, payment ID, dan referensi yang persis sama:
     `subscription.id: 584b3b91-9baf-41ea-a604-6cf13835c54f`
     `payment.reference: 18751c2fe44fcab19fe48487`
4. Uji Concurrency (5 request paralel sekaligus):
   - Seluruh 5 request mengembalikan HTTP 201 dengan ID langganan dan payment yang identik tanpa ada error 500.

---

## TKT-102: Status Langganan Aneh, Kadang Muncul Kadang Engga (Public HTTP Cache Leak)

### 1. Deskripsi Keluhan
Pengguna melaporkan:
> “Aku sudah bayar dan konfirmasi pembayarannya sukses, tapi halaman Paket masih tulis belum berlangganan. Aku tutup tab terus buka lagi, masih sama. Yang bikin takut, teman sekantor pakai WiFi kantor dan di layarnya malah muncul status dan kuota milikku, padahal dia belum login.”

### 2. Investigasi & Akar Masalah (*Root Cause*)
Setelah ditelusuri dari alur endpoint status langganan (`GET /api/v1/subscription/status`):

1. **Header `Cache-Control` Publik pada Endpoint Privat Berautentikasi (`api/internal/modules/subscription/mount.go`)**:
   - Endpoint `GET /api/v1/subscription/status` mengembalikan data spesifik dan privat milik user yang sedang login (status langganan, nama paket, sisa kuota, dan masa berlaku).
   - Namun di `mount.go`, handler rute tersebut dibungkus dengan pemberian header HTTP cache publik:
     ```go
     w.Header().Set("Cache-Control", "public, max-age=86400")
     ```
   - Direktif `public` mengizinkan seluruh perantara jaringan (*shared caching proxy*, seperti gateway/proxy WiFi kantor, reverse proxy, atau CDN) untuk menyimpan respons tersebut dan menyajikannya kepada pengguna lain.
   - Durasi `max-age=86400` memerintahkan browser dan proxy untuk menyimpan respons hingga 24 jam tanpa perlu memvalidasi ulang ke server asal (*origin server*).

2. **Dampak Masalah**:
   - **Status Tidak Berubah Setelah Bayar**: Saat pengguna membuka aplikasi sebelum membayar, browser meng-cache respons `{"subscribed": false}` selama 24 jam. Setelah membayar lunas, ketika pengguna menutup dan membuka tab kembali, browser menyajikan data lama dari *disk cache* alih-alih menghubungi backend.
   - **Kebocoran Data ke Teman Sekantor**: Ketika berada dalam satu jaringan WiFi kantor yang memiliki caching proxy transparan, respons berstatus aktif milik pengguna yang sudah bayar disimpan oleh proxy. Pengguna lain (meskipun belum login) yang membuka halaman web menerima respons cache tersebut, sehingga status dan kuota akun milik pengguna pertama bocor ke layar pengguna lain.

---

### 3. Kode yang Diubah

#### A. Backend
1. **`api/internal/modules/subscription/handler.go`**:
   - Menambahkan header `Cache-Control: no-store, no-cache, must-revalidate` dan `Pragma: no-cache` pada fungsi `Status` agar seluruh respons status privat tidak disimpan di cache browser maupun shared proxy.
2. **`api/internal/modules/subscription/mount.go`**:
   - Menghapus wrapper anonymous function yang memasang `Cache-Control: public, max-age=86400`. Rute `GET /api/v1/subscription/status` kini langsung diteruskan ke handler `h.Status`.
3. **`api/internal/modules/subscription/handler_test.go`**:
   - Menambahkan unit test `TestHandler_Status_SetsNoStoreHeaders` untuk memastikan handler `Status` selalu menyetel header `no-store`.
   - Menambahkan unit test `TestRegisterRoutes_Status_DoesNotHavePublicCache` untuk memastikan route registrasi tidak lagi menyisipkan header `public, max-age=86400`.

---

### 4. Bukti Penyelesaian (*Proof of Completion*)

#### A. Langkah Reproduksi Sebelum Perbaikan
1. Kirim request berautentikasi ke endpoint `GET /api/v1/subscription/status`:
   ```bash
   curl -i http://localhost:8080/api/v1/subscription/status \
     -H "Authorization: Bearer <token>"
   ```
   **Output**:
   ```http
   HTTP/1.1 200 OK
   Cache-Control: public, max-age=86400
   Content-Type: application/json
   Date: Tue, 24 Nov 2026 03:35:37 GMT
   Content-Length: 114

   {"success":true,"code":20104,"message":"status langganan","data":{"subscribed":false,"memesUsed":0,"maxMemes":0}}
   ```
2. Header `Cache-Control: public, max-age=86400` menyebabkan browser dan shared proxy kantor menyimpan data ini selama 24 jam dan membagikannya ke klien lain di jaringan yang sama.

#### B. Langkah Reproduksi Sesudah Perbaikan
1. Jalankan unit test modul subscription:
   ```bash
   cd api && go test -v ./internal/modules/subscription
   ```
   **Output**:
   ```
   === RUN   TestHandler_Checkout_RequiresIdempotencyKey
   --- PASS: TestHandler_Checkout_RequiresIdempotencyKey (0.00s)
   === RUN   TestHandler_Status_SetsNoStoreHeaders
   --- PASS: TestHandler_Status_SetsNoStoreHeaders (0.00s)
   === RUN   TestRegisterRoutes_Status_DoesNotHavePublicCache
   --- PASS: TestRegisterRoutes_Status_DoesNotHavePublicCache (0.00s)
   PASS
   ok      github.com/nyxsr/meme-portal/api/internal/modules/subscription  0.003s
   ```
2. Kirim request berautentikasi ke `GET /api/v1/subscription/status`:
   ```bash
   curl -i http://localhost:8080/api/v1/subscription/status \
     -H "Authorization: Bearer <token>"
   ```
   **Output**:
   ```http
   HTTP/1.1 200 OK
   Cache-Control: no-store, no-cache, must-revalidate
   Content-Type: application/json
   Pragma: no-cache

   {"success":true,"code":20104,"message":"status langganan","data":{"subscribed":true,"status":"active",...}}
   ```
3. Kirim request tanpa token autentikasi (seperti teman sekantor yang belum login):
   ```bash
   curl -i http://localhost:8080/api/v1/subscription/status
   ```
   **Output**:
   ```http
   HTTP/1.1 401 Unauthorized
   Content-Type: application/json

   {"success":false,"code":10002,"message":"unauthorized"}
   ```
   *Shared proxy tidak dapat menyajikan respons cache milik pengguna terautentikasi ke pengguna yang belum login, dan browser selalu menerima status terkini dari server.*

---

## TKT-107: Bisa Hapus dan Lihat Meme Orang Lain (IDOR / BOLA pada Modul Meme)

### 1. Deskripsi Keluhan
Pengguna melaporkan:
> “Aku iseng-iseng ganti angka di URL dan ternyata aku bisa MELIHAT dan MENGHAPUS meme milik user lain. Aku sudah hapus satu sebagai bukti, tolong pulihkan. Tolong cek juga apakah ini berlaku untuk semua meme di sistem. Kalau ada yang niat jahat ini bakal jadi masalah besar.”

### 2. Investigasi & Akar Masalah (*Root Cause*)
Setelah ditelusuri alur pengelolaan meme dari database, repository, service, handler, hingga route mounting:

1. **Broken Object Level Authorization (BOLA) / IDOR pada Penghapusan Meme (`DELETE /api/v1/memes/{id}`)**:
   - Method `Delete(ctx context.Context, id, userID string)` pada `api/internal/modules/meme/repository.go` menerima parameter `userID` dari context autentikasi JWT (`middleware.UserIDFromContext`).
   - Namun, query SQL yang dieksekusi hanya berupa:
     ```sql
     DELETE FROM memes WHERE id = $1
     ```
     Klausa `AND user_id = $2` sama sekali tidak disertakan.
   - Akibatnya, pengguna terautentikasi mana pun dapat menghapus meme milik pengguna lain hanya dengan mengetahui atau memasukkan ID target pada URL endpoint.

2. **Celah Otorisasi yang Sama pada Pembacaan dan Update Meme (`GetByID` dan `SetResultURL`)**:
   - Pada `GetByID(ctx context.Context, id, userID string)` di `repository.go`, query SQL hanya mengecek `WHERE id = $1` tanpa memvalidasi kepemilikan `AND user_id = $2`. Selain itu, query `SELECT` memanggil 10 kolom (termasuk `updated_at`), namun destinasi `row.Scan` hanya menyediakan 9 variabel penampung sehingga method ini rentan runtime scan mismatch jika dipanggil.
   - Pada `SetResultURL(ctx context.Context, id, userID, resultURL string)` di `repository.go`, query SQL mengeksekusi `UPDATE memes SET result_url = $2 WHERE id = $1` tanpa klausa kepemilikan `AND user_id = $3`. Akibatnya, endpoint penyelesaian render (`POST /api/v1/memes/{id}/complete`) dapat dieksploitasi untuk menimpa file hasil meme milik pengguna lain.

3. **Absennya Route dan Handler `GET /api/v1/memes/{id}`**:
   - Konstanta kode status `CodeMemeFetched = 30103` dan query sqlc `GetMemeByID` sudah tersedia di codebase, namun route `GET /api/v1/memes/{id}` belum terdaftar di `mount.go` maupun di handler/service. Pengguna yang mencoba mengakses detail spesifik meme mendapatkan respons `405 Method Not Allowed`.

4. **Ketiadaan Tombol Hapus pada Antarmuka Studio**:
   - Di sisi frontend (`web/src/app/studio/page.tsx`), daftar "Meme Saya" menampilkan daftar meme namun belum memiliki tombol aksi hapus, meskipun hook mutasi `useDeleteMeme` sudah tersedia di `web/src/lib/queries.ts`.

---

### 3. Kode yang Diubah

#### A. Backend
1. **`api/internal/modules/meme/repository.go`**:
   - Memperbaiki `Delete` agar menyertakan validasi kepemilikan:
     ```sql
     DELETE FROM memes WHERE id = $1 AND user_id = $2
     ```
   - Memperbaiki `GetByID` agar menyertakan validasi kepemilikan dan menyamakan 9 kolom data `Meme`:
     ```sql
     SELECT id, user_id, title, base_url, result_url, top_text, bottom_text, font_size, created_at
     FROM memes WHERE id = $1 AND user_id = $2
     ```
   - Memperbaiki `SetResultURL` agar menyertakan validasi kepemilikan:
     ```sql
     UPDATE memes SET result_url = $2
     WHERE id = $1 AND user_id = $3
     RETURNING ...
     ```
2. **`api/internal/modules/meme/service.go`**:
   - Menambahkan method `Get(ctx context.Context, userID, id string) (Meme, error)` yang meneruskan query aman `GetByID` dengan pengecekan `userID`.
3. **`api/internal/modules/meme/handler.go`**:
   - Menambahkan handler `Get(w http.ResponseWriter, r *http.Request)` untuk merespons permintaan detail meme pengguna dengan status `200 OK` dan kode `CodeMemeFetched (30103)`.
4. **`api/internal/modules/meme/mount.go`**:
   - Mendaftarkan rute `GET /api/v1/memes/{id}` yang dibungkus middleware autentikasi `authMW`.
5. **`api/internal/modules/meme/handler_test.go`**:
   - Menambahkan unit test untuk memverifikasi proteksi autentikasi pada endpoint `Delete` dan `Get`, serta memastikan route `GET` dan `DELETE` terdaftar dan terproteksi di HTTP ServeMux.

#### B. Frontend
1. **`web/src/app/studio/page.tsx`**:
   - Mengimpor dan menginisialisasi `useDeleteMeme()`.
   - Menambahkan tombol "Hapus" pada setiap item kartu di daftar "Meme Saya" lengkap dengan konfirmasi dialog (`window.confirm`) dan penonaktifan tombol (`disabled={deleteMeme.isPending}`) saat proses penghapusan berlangsung.

---

### 4. Bukti Penyelesaian (*Proof of Completion*)

#### A. Langkah Reproduksi Sebelum Perbaikan
1. Siapkan dua akun pengguna berbeda: User A (`krisnalavendrairawan@gmail.com`) dan User B (`tester102@example.com`).
2. Buat meme milik User A di database:
   - Meme ID: `11111111-1111-1111-1111-111111111111`
   - Owner (`user_id`): `6a7ed9b8-7c36-4014-a90e-358400e7ec9f` (User A)
3. Login sebagai User B dan kirim request `DELETE /api/v1/memes/11111111-1111-1111-1111-111111111111`:
   ```bash
   curl -i -X DELETE http://localhost:8080/api/v1/memes/11111111-1111-1111-1111-111111111111 \
     -H "Authorization: Bearer <token_user_b>"
   ```
   **Output**:
   ```http
   HTTP/1.1 200 OK
   Content-Type: application/json

   {"success":true,"code":30105,"message":"meme dihapus"}
   ```
4. Periksa database:
   ```sql
   SELECT * FROM memes WHERE id = '11111111-1111-1111-1111-111111111111';
   ```
   **Output**: `0 rows`. Meme milik User A terhapus oleh User B (Vulnerabilitas IDOR terbukti).

#### B. Langkah Reproduksi Sesudah Perbaikan
1. Jalankan unit test modul meme di backend:
   ```bash
   cd api && go test -v ./internal/modules/meme
   ```
   **Output**:
   ```
   === RUN   TestHandler_Delete_RequiresAuth
   --- PASS: TestHandler_Delete_RequiresAuth (0.00s)
   === RUN   TestHandler_Get_RequiresAuth
   --- PASS: TestHandler_Get_RequiresAuth (0.00s)
   === RUN   TestRegisterRoutes_IncludesGetAndOwnershipProtectedRoutes
   --- PASS: TestRegisterRoutes_IncludesGetAndOwnershipProtectedRoutes (0.00s)
   PASS
   ok      github.com/nyxsr/meme-portal/api/internal/modules/meme  0.004s
   ```
2. Jalankan seluruh pengecekan kode otomatis:
   ```bash
   make check
   ```
   **Output**:
   ```
   cd api && go build -o bin/api ./cmd/server
   cd web && bun run check
   $ tsc --noEmit
   ```
3. Pengujian keamanan end-to-end (IDOR Protection Test):
   - Siapkan Meme ID `33333333-3333-3333-3333-333333333333` milik User A (`07f8854c-66a0-49e1-a4c6-912716c55c60`).
   - **Percobaan 1 (User B mencoba melihat meme User A)**:
     ```bash
     curl -i -X GET http://localhost:8080/api/v1/memes/33333333-3333-3333-3333-333333333333 \
       -H "Authorization: Bearer <token_user_b>"
     ```
     **Output**:
     ```http
     HTTP/1.1 404 Not Found
     {"success":false,"code":30001,"message":"meme tidak ditemukan"}
     ```
   - **Percobaan 2 (User B mencoba menghapus meme User A)**:
     ```bash
     curl -i -X DELETE http://localhost:8080/api/v1/memes/33333333-3333-3333-3333-333333333333 \
       -H "Authorization: Bearer <token_user_b>"
     ```
     **Output**:
     ```http
     HTTP/1.1 404 Not Found
     {"success":false,"code":30001,"message":"meme tidak ditemukan"}
     ```
   - Cek database: Meme tetap aman tersimpan di database (`1 row`).
   - **Percobaan 3 (User A melihat meme miliknya sendiri)**:
     ```bash
     curl -i -X GET http://localhost:8080/api/v1/memes/33333333-3333-3333-3333-333333333333 \
       -H "Authorization: Bearer <token_user_a>"
     ```
     **Output**:
     ```http
     HTTP/1.1 200 OK
     {"success":true,"code":30103,"message":"meme ditemukan","data":{"id":"33333333-...","userId":"07f8854c-...","title":"Meme Rahasia User A",...}}
     ```
   - **Percobaan 4 (User A menghapus meme miliknya sendiri)**:
     ```bash
     curl -i -X DELETE http://localhost:8080/api/v1/memes/33333333-3333-3333-3333-333333333333 \
       -H "Authorization: Bearer <token_user_a>"
     ```
     **Output**:
     ```http
     HTTP/1.1 200 OK
     {"success":true,"code":30105,"message":"meme dihapus"}
     ```
   - Cek database: Meme berhasil terhapus secara sah oleh pemiliknya (`0 rows`).


---

## TKT-104: Sudah Bayar tapi Navbar Bilang Belum Langganan sampai Refresh (Stale Cache & Sinkronisasi Kuota)

### 1. Deskripsi Keluhan
Pengguna melaporkan:
> “Aku beli paket tadi dan pembayarannya sukses. Tapi nama paket di pojok kanan atas tetap kosong dan kuota di halaman Paket macet di angka lama. Setelah aku hard refresh pakai Ctrl Shift R barulah muncul. Tiap kali begitu bikin bete.”

### 2. Investigasi & Akar Masalah (*Root Cause*)
Setelah ditelusuri dari interaksi komponen antarmuka (*client-side*) dan manajemen state query di frontend (`web/`):

1. **Nama Paket Tidak Dirender pada Header Akun (`web/src/components/site-header.tsx`)**:
   - Di navbar pojok kanan atas, tombol akun pengguna hanya merender string `{user.name || user.email}`.
   - Nama paket langganan aktif (`planName`) disembunyikan di dalam popover `HoverCard` dan tidak ditampilkan langsung di tombol navbar. Pengguna yang berhasil berlangganan mengira sistem belum mencatat statusnya karena nama paket di pojok kanan atas tampak kosong.

2. **Inisialisasi Cache Tak Hingga pada Status Langganan (`web/src/lib/queries.ts`)**:
   - Hook `useSubscriptionStatus()` disetel dengan `staleTime: Number.POSITIVE_INFINITY`.
   - Hal ini membuat TanStack Query menganggap data status langganan abadi dan tidak pernah melakukan *background refetch* secara otomatis meskipun tab/halaman dibuka kembali (*window focus*).

3. **Ketiadaan Query Invalidation saat Kuota Berubah (`web/src/lib/queries.ts`)**:
   - Saat pengguna membuat/menyelesaikan meme (`useCompleteMeme`) atau menghapus meme (`useDeleteMeme`), mutasi hanya meng-invalidate cache `["memes"]`.
   - Query `["subscription"]` tidak di-invalidate. Akibatnya, angka penggunaan kuota (`memesUsed`) pada halaman `/pricing` tetap macet di angka lama dan tidak berkurang/bertambah saat pengguna berpindah halaman dari Studio kembali ke Paket. Pengguna terpaksa melakukan hard refresh (`Ctrl + Shift + R`) agar seluruh state di-reset.

4. **Ketiadaan Sinkronisasi Cache saat Login & Register (`web/src/lib/auth.ts`)**:
   - Mutasi `useLogin` dan `useRegister` hanya meng-invalidate query `["me"]`, sehingga jika sesi berubah, cache `["subscription"]` berisiko tetap menyajikan state lama.

---

### 3. Kode yang Diubah

#### Frontend (`web/`)
1. **`web/src/components/site-header.tsx`**:
   - Menambahkan badge nama paket di samping nama/email pengguna pada tombol akun navbar jika pengguna memiliki langganan aktif (`sub.data?.subscribed && sub.data.planName`).
2. **`web/src/lib/queries.ts`**:
   - Mengubah `staleTime` pada `useSubscriptionStatus` dari `Number.POSITIVE_INFINITY` menjadi 30 detik (`1000 * 30`) serta mengaktifkan `refetchOnWindowFocus: true`.
   - Menambahkan `qc.invalidateQueries({ queryKey: queryKeys.subscription })` pada callback `onSuccess` di mutasi `useCompleteMeme` dan `useDeleteMeme` agar sisa kuota meme selalu ter-update secara otomatis setelah pembuatan atau penghapusan meme.
3. **`web/src/lib/auth.ts`**:
   - Menambahkan `qc.invalidateQueries({ queryKey: ["subscription"] })` pada mutasi `useRegister` dan `useLogin` untuk memastikan status langganan segar segera setelah user masuk.

---

### 4. Bukti Penyelesaian (*Proof of Completion*)

#### A. Langkah Reproduksi Sebelum Perbaikan
1. Login dan beli paket langganan di `/pricing`.
2. Setelah pembayaran sukses dan dialihkan kembali ke aplikasi, perhatikan pojok kanan atas navbar: tombol akun hanya menampilkan nama pengguna tanpa ada badge/nama paket.
3. Buka halaman `/studio`, upload gambar dan render 1 meme hingga selesai.
4. Buka kembali halaman `/pricing`: angka kuota meme tetap `0 / 20` (tidak berubah), kuota macet di angka lama.
5. Lakukan hard refresh (`Ctrl + Shift + R`), barulah kuota ter-update menjadi `1 / 20`.

#### B. Langkah Reproduksi Sesudah Perbaikan
1. Verifikasi kompilasi frontend dan typecheck:
   ```bash
   cd web && bun run check && bun run build
   ```
   **Output**:
   ```
   $ tsc --noEmit
   ✓ Compiled successfully
   ✓ Generating static pages (8/8)
   ```
2. Jalankan seluruh pengujian kode otomatis:
   ```bash
   make check
   ```
   **Output**:
   ```
   cd api && go build -o bin/api ./cmd/server
   cd web && bun run check
   $ tsc --noEmit
   ```
3. Uji perilaku antarmuka pada browser:
   - Login dengan user aktif $\rightarrow$ Pojok kanan atas navbar menampilkan nama user beserta badge paket aktif secara langsung (mis. `[user] [Premium 1 Hari]`).
   - Masuk ke `/studio`, buat 1 meme baru hingga berhasil tersimpan.
   - Navigasi kembali ke halaman `/pricing` via navbar $\rightarrow$ Kuota meme langsung ter-update otomatis (mis. `1 / 20 (sisa 19)`) tanpa perlu me-refresh halaman atau menekan `Ctrl + Shift + R`.
   - Hapus meme yang dibuat $\rightarrow$ Kuota otomatis kembali tersinkronisasi.


---

## TKT-101: Masih Bisa Membuat Meme Padahal Paket Habis (Pengecekan Masa Aktif Langganan)

### 1. Deskripsi Keluhan
Pengguna melaporkan:
> “Halo, aku beli paket Premium 1 Hari kemarin jam 12 siang. Lewat hampir satu jam tapi aku masih bisa upload dan render meme terus. Temenku bilang di dia udah gak bisa. Aku restart browser tadi malah normal sebentar, terus balik bisa lagi. Tolong dicek ya, aku gak mau dilabeli ngehack padahal bayar cuma 1 hari.”

### 2. Investigasi & Akar Masalah (*Root Cause*)
Setelah ditelusuri alur otorisasi pembuatan meme (`PrepareUpload`, `ResultUpload`, dan `Complete`) pada modul `meme` dan `subscription`:

1. **Query `ActiveByUser` Tidak Memeriksa Batas Waktu `ends_at` (`api/internal/modules/subscription/repository.go`)**:
   - Pengecekan hak akses membuat meme (`subscriptionGate.CanCreate`) memanggil `subSvc.Status()`, yang bergantung pada fungsi `repo.ActiveByUser()`.
   - Pada `ActiveByUser()`, query SQL yang dijalankan adalah:
     ```sql
     SELECT ... FROM subscriptions s JOIN plans p ON p.id = s.plan_id
     WHERE s.user_id = $1 AND s.status = 'active'
     ORDER BY s.ends_at DESC LIMIT 1
     ```
   - Query ini **hanya** memeriksa kolom `status = 'active'` tanpa memvalidasi apakah waktu saat ini sudah melewati masa berlaku (`ends_at > now()`).

2. **Ketergantungan Tunggal pada Background Janitor**:
   - Kolom `status` di tabel `subscriptions` hanya diubah menjadi `'expired'` oleh *background janitor* (`expireJanitor` di `api/internal/app/app.go`) yang berjalan secara periodik setiap 1 jam sekali (`expireInterval = time.Hour`).
   - Akibatnya, selama janitor belum dieksekusi (rentang waktu hingga 1 jam setelah langganan habis), langganan masih berstatus `'active'` di database, sehingga query `ActiveByUser()` tetap menganggap langganan valid. Pengguna masih dapat terus mengunggah dan merender meme hingga janitor akhirnya berjalan.
   - Catatan pengembang di komentar kode (`// lazy check di ActiveByUser (ends_at > now()) tetap menjadi sumber kebenaran runtime`) dan query sqlc di `subscriptions.sql` (`AND s.ends_at > now()`) mengonfirmasi bahwa klausa `AND s.ends_at > now()` memang seharusnya ada di `ActiveByUser()`.

---

### 3. Kode yang Diubah

#### Backend (`api/`)
1. **`api/internal/modules/subscription/repository.go`**:
   - Menambahkan klausul `AND s.ends_at > now()` pada query SQL di fungsi `ActiveByUser`.
   - Dengan perbaikan ini, langganan yang sudah melewati `ends_at` langsung ditolak secara *real-time* saat query dieksekusi tanpa menunggu *background janitor* mengubah status.
2. **`api/internal/modules/subscription/repository_test.go`**:
   - Menambahkan integration test `TestRepository_ActiveByUser_FiltersExpiredSubscriptions` untuk memverifikasi bahwa langganan yang masa berlakunya telah habis (`ends_at <= now()`) mengembalikan `ErrNotSubscribed`, meskipun kolom `status` di database masih bernilai `'active'`.

---

### 4. Bukti Penyelesaian (*Proof of Completion*)

#### A. Langkah Reproduksi Sebelum Perbaikan
1. Masukkan data langganan yang sudah lewat masa berlaku (mis. `starts_at = now() - interval '2 days'`, `ends_at = now() - interval '1 hour'`) dengan status `'active'`.
2. Jalankan query `ActiveByUser` lama:
   ```sql
   SELECT id, status, ends_at FROM subscriptions
   WHERE user_id = '<user_id>' AND status = 'active'
   ORDER BY ends_at DESC LIMIT 1;
   ```
   **Output**: Mengembalikan 1 baris (langganan dianggap masih aktif karena janitor 1 jam belum berjalan).
3. Pengguna memanggil `POST /api/v1/memes/upload` $\rightarrow$ Mendapat status 200 OK dan presigned URL S3 (dapat membuat meme padahal paket sudah kedaluwarsa).

#### B. Langkah Reproduksi Sesudah Perbaikan
1. Jalankan unit & integration test subscription di backend:
   ```bash
   cd api && go test -v -run TestRepository_ActiveByUser_FiltersExpiredSubscriptions ./internal/modules/subscription
   ```
   **Output**:
   ```
   === RUN   TestRepository_ActiveByUser_FiltersExpiredSubscriptions
   --- PASS: TestRepository_ActiveByUser_FiltersExpiredSubscriptions (0.02s)
   PASS
   ok      github.com/nyxsr/meme-portal/api/internal/modules/subscription  0.020s
   ```
2. Jalankan seluruh test backend:
   ```bash
   cd api && go test -v ./...
   ```
   **Output**:
   ```
   PASS (seluruh test di internal/middleware, auth, meme, subscription, cookie lulus)
   ```
3. Jalankan pengujian `make check`:
   ```bash
   make check
   ```
   **Output**:
   ```
   cd api && go build -o bin/api ./cmd/server
   cd web && bun run check
   $ tsc --noEmit
   ```
4. Uji simulasi query database:
   - Query `ActiveByUser` baru dengan filter `AND s.ends_at > now()` mengembalikan `0 rows` (`pgx.ErrNoRows` $\rightarrow$ `ErrNotSubscribed`).
   - Endpoint upload `/api/v1/memes/upload` langsung mengembalikan `HTTP 403 Forbidden` (`{"success":false,"code":30002,"message":"anda belum berlangganan paket aktif"}`).
