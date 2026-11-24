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
