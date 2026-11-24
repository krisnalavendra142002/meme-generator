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
