# ☘️ lenwy-whatsmeow

**lenwy-whatsmeow** adalah library WhatsApp API untuk Node.js dengan performa tinggi. Library ini punya gaya pemakaian (developer experience) yang mirip **Baileys** (`makeWASocket`, `ev.on`, `sendMessage`, dll), tapi di balik layar, seluruh koneksi ke WhatsApp ditangani oleh engine **Go (`whatsmeow`)** yang jauh lebih ringan dan cepat.

Jadi kamu tetap menulis kode dengan sintaks yang familiar, tapi mendapat keuntungan kecepatan & efisiensi memori dari Go.

---

## Kenapa Pakai lenwy-whatsmeow?

- **Sintaks familiar** — kalau kamu pernah pakai Baileys, kamu tidak perlu belajar API baru dari nol.
- **Ringan & cepat** — koneksi WebSocket ke WhatsApp ditangani oleh Go, bukan JavaScript.
- **Auto reconnect** — kalau internet putus, koneksi otomatis dipulihkan tanpa perlu scan ulang.
- **Aman untuk sesi** — proses shutdown (`Ctrl+C`, dsb) ditangani dengan baik supaya database sesi tidak corrupt.
- **Fitur media siap pakai** — kirim/download media, convert ke voice note, buat stiker (statis & animasi), kompres gambar, sampai ubah stiker balik jadi gambar/video — semua sudah dibungkus jadi satu perintah.
- **Info & Interaksi Lengkap** — cek status kontak, gabung/keluar grup lewat link, presence (online/mengetik), foto profil, dan lainnya.

---

## Instalasi

Install library langsung via NPM atau Yarn:

```bash
npm install lenwy-whatsmeow
```

atau menggunakan Yarn:

```Bash
yarn add lenwy-whatsmeow
```
## Zero Go Dependency (Siap Pakai di Pterodactyl)

Library ini tidak membutuhkan instalasi Go di server atau komputer kamu!

Binary hasil kompilasi Go untuk Linux (main_linux) dan Windows (main_win.exe) sudah dibundel langsung di dalam paket. Kamu bisa langsung menjalankannya di platform hosting berbasis Node.js seperti Pterodactyl (Egg Node.js), VPS, maupun komputer lokal tanpa setup tambahan.

Prasyarat

- Node.js: Versi LTS terbaru (v18+ disarankan).

- FFmpeg & Media Suite: Sudah ditangani otomatis lewat ffmpeg-static dan sharp bawaan paket (tidak perlu install FFmpeg secara manual).

---

## Pretty Chat Logger (chatLog)

Menampilkan log pesan masuk di terminal secara rapi, berwaktu, dan berwarna menggunakan badge.
JavaScript

```js
sock.ev.on("messages.upsert", ({ meta, raw }) => {
  chatLog({ ...meta, timestamp: raw.timestamp, isFromMe: raw.isFromMe, pushName: meta.pushname });
});
```
---

## Mulai Cepat (Quick Start)

Berikut contoh dasar untuk menghubungkan bot ke WhatsApp:

```javascript
import { makeWASocket } from "lenwy-whatsmeow";

const conn = makeWASocket({
  sessionName: "my-session", // Nama sesi, bebas kamu tentukan
});

// PENTING: Wajib Dipanggil Secara Manual!
// makeWASocket() Hanya Menyiapkan Instance, Belum Menyalakan Proses Go Engine-nya.
// Tanpa Ini, Semua Method Seperti requestPairingCode() Akan Gagal Dengan Error
// "Go process belum berjalan."
conn.start();

// Status koneksi
conn.ev.on("connection.update", (data) => {
  if (data.reason === "connection_lost") {
    console.log("⚠️ Koneksi terputus, mencoba menyambung ulang...");
  }

  if (data.open) {
    console.log("✅ Berhasil terhubung ke WhatsApp!");
  }
});

// Kode pairing (kalau login pakai nomor HP, bukan QR)
conn.ev.on("pairing_code", (code) => {
  console.log("Kode Pairing Kamu:", code);
});

// Pesan masuk
conn.ev.on("messages.upsert", (m) => {
  console.log("Pesan Masuk:", m);
});

// Minta kode pairing (panggil SETELAH conn.start(), boleh langsung tanpa delay
// tambahan karena perintah akan otomatis antre sampai proses Go-nya siap)
await conn.requestPairingCode("628xxxxxxxxxx");
```

> **Catatan:** Nama fungsi/opsi seperti `sessionName` mengikuti pola umum yang dipakai di seluruh contoh dokumentasi ini.

---

## Mengirim Pesan

Semua jenis pesan dikirim lewat `conn.sendMessage(jid, content, options)`.

```javascript
// Kirim Pesan Teks
await conn.sendMessage(jid, {
  text: "Halo World",
});

// Kirim Gambar
await conn.sendMessage(jid, {
  image: "./foto.jpg",
  caption: "Deskripsi Gambar",
});

// Kirim Video
await conn.sendMessage(jid, {
  video: "./video.mp4",
  caption: "Deskripsi Video",
});

// Kirim Video Sebagai GIF (Auto-Loop, Tanpa Suara)
await conn.sendMessage(jid, {
  video: "./animasi.mp4",
  gifPlayback: true,
});

// Kirim Gambar/Video Langsung Dari URL
await conn.sendMessage(jid, {
  image: { url: "https://example.com/image.jpg" },
  caption: "Kirim langsung dari CDN!"
});

// Kirim Stiker
await conn.sendMessage(jid, {
  sticker: "./stiker.webp",
});

// Kirim Audio Biasa
await conn.sendMessage(jid, {
  audio: "./lagu.mp3",
});

// Kirim Voice Note (PTT)
await conn.sendMessage(jid, {
  ptt: "./vn.ogg",
});

// Kirim Dokumen
await conn.sendMessage(jid, {
  document: "./file.pdf",
  fileName: "Modul_Belajar.pdf",
});

// Membalas Pesan (Quoted Reply)
await conn.sendMessage(
  jid,
  { text: "Ini balasan pesan" },
  { quoted: m },
);
```

---

## Reaksi, Hapus, dan Edit Pesan

Setiap pesan masuk (`m`) sudah otomatis membawa info pesan yang di-reply (kalau ada) — kamu **tidak perlu lagi** menulis fungsi pencarian manual. Library menyimpan cache pesan terakhir secara internal, jadi walaupun pesan yang di-reply itu sudah agak lama, `m.quotedText` dan `m.quotedType` tetap bisa terisi otomatis.

```javascript
sock.ev.on("messages.upsert", ({ m }) => {
  if (m.quotedId) {
    const quoted = { id: m.quotedId, participant: m.quotedSender };

    // Kasih Reaksi Emoji
    await conn.react(jid, quoted, "☘️");

    // Hapus Pesan
    await conn.deleteMessage(jid, quoted);

    // Edit Pesan
    await conn.editMessage(jid, { id: quoted.id }, "Pesan Ini Telah Diperbarui");

    // Bonus: Isi Teks & Tipe Pesan Yang Di-Reply, Otomatis Dari Cache Internal
    console.log("Teks Yang Di-Reply:", m.quotedText);
    console.log("Tipe Pesan Yang Di-Reply:", m.quotedType); // "Chat", "Image", dsb
  }
});
```

**Field yang tersedia di `m` untuk pesan yang di-reply:**

| Field | Isi |
| :--- | :--- |
| `m.quotedId` | ID Pesan Yang Di-Reply |
| `m.quotedSender` | JID Pengirim Pesan Yang Di-Reply |
| `m.quotedType` | Tipe Pesan Yang Di-Reply (`Chat`, `Image`, dll) |
| `m.quotedText` | Isi Teks Pesan Yang Di-Reply (Diambil Dari Cache Internal Kalau Tidak Terkirim Langsung) |

> **Catatan:** Kalau kamu masih pakai fungsi `getQuotedInfo()` custom dari versi sebelumnya, itu tetap berfungsi (karena `raw.quotedId`/`raw.quotedSender` tetap ada), tapi sudah tidak wajib lagi untuk kasus pemakaian umum.

---

## Manajemen Grup

### Cek Info Grup & Status Admin Bot

```javascript
const metadata = await conn.groupMetadata(groupJid);

console.log(metadata.subject);     // Nama grup
console.log(metadata.isBotAdmin);  // true / false
```

### Kelola Anggota

```javascript
// Keluarkan anggota
await conn.groupParticipantsUpdate(groupJid, ["628xxx@s.whatsapp.net"], "remove");

// Jadikan admin
await conn.groupParticipantsUpdate(groupJid, ["628xxx@s.whatsapp.net"], "promote");

// Turunkan dari admin
await conn.groupParticipantsUpdate(groupJid, ["628xxx@s.whatsapp.net"], "demote");

// Tambah anggota
const res = await conn.groupParticipantsUpdate(groupJid, ["628xxx@s.whatsapp.net"], "add");

// Kalau target mengaktifkan privasi "siapa yang bisa menambahkanku ke grup",
// bot tidak bisa menambahkan secara langsung — kirim link undangan sebagai gantinya
if (res?.inviteRequired) {
  console.log("Kirimkan link undangan ini:", res.inviteLink);
}
```

### Pengaturan Grup Lainnya

```javascript
// Kunci grup (hanya admin yang bisa kirim pesan)
await conn.groupSettingUpdate(groupJid, "announcement");

// Buka kunci grup
await conn.groupSettingUpdate(groupJid, "not_announcement");

// Ganti nama grup
await conn.groupUpdateSubject(groupJid, "Nama Grup Baru");

// Ganti deskripsi grup
await conn.groupUpdateDescription(groupJid, "Deskripsi baru grup ini.");

// Ambil link undangan
const inviteLink = await conn.groupInviteCode(groupJid);

// Reset / buat link undangan baru
const newInviteLink = await conn.groupRevokeInviteCode(groupJid);
```

### Gabung & Keluar Grup Lewat Link

```javascript
// Lihat Info Grup Dari Link Undangan (Tanpa Ikut Gabung Dulu)
const groupInfo = await conn.getGroupInfoFromLink("kodeInviteDisini");
console.log(groupInfo);

// Gabung Ke Grup Pakai Kode Undangan
const joinResult = await conn.joinGroupWithLink("kodeInviteDisini");
console.log(joinResult);

// Keluar Dari Grup
await conn.leaveGroup(groupJid);

// Lihat Semua Grup Yang Sudah Diikuti Bot
const groups = await conn.getJoinedGroups();
console.log(groups);
```

---

## Info Kontak & Akun

```javascript
// Cek Info User (Bisa Satu JID Atau Array JID)
const info = await conn.getUserInfo(["628xxx@s.whatsapp.net"]);
console.log(info);

// Cek Apakah Nomor Terdaftar Di WhatsApp
const checkResult = await conn.isOnWhatsApp(["628xxxxxxxxxx"]);
console.log(checkResult);

// Ambil Info Akun Bisnis (Kalau Kontaknya Akun Bisnis)
const business = await conn.getBusinessProfile(jid);
console.log(business);

// Ambil URL Foto Profil
const avatarUrl = await conn.profilePictureUrl(jid);
console.log(avatarUrl);
```

> Bentuk data yang dikembalikan method-method di atas mengikuti struktur bawaan `whatsmeow` — silakan `console.log()` hasilnya untuk melihat field yang tersedia sesuai kebutuhan kamu.

---

## Presence (Status Online & Mengetik)

```javascript
// Set Status Kamu (Bot) Jadi Online/Offline
await conn.sendPresence("available");   // Online
await conn.sendPresence("unavailable"); // Offline

// Tampilkan Indikator "Sedang Mengetik..." Di Chat Tertentu
await conn.sendChatPresence(jid, "composing", "text");

// Tampilkan Indikator "Sedang Merekam Audio..."
await conn.sendChatPresence(jid, "recording", "audio");

// Hentikan Indikator (Kembali Netral)
await conn.sendChatPresence(jid, "paused");

// Berlangganan Update Presence Kontak Tertentu (Online/Terakhir Dilihat)
await conn.subscribePresence(jid);
```

---

## Tandai Pesan Sudah Dibaca

```javascript
// Bentuk Sederhana (Positional Arguments)
await conn.markRead(["idPesan1", "idPesan2"], timestamp, chatJid, senderJid);

// Atau Bentuk Object (Lebih Jelas)
await conn.markRead({
  ids: ["idPesan1", "idPesan2"],
  timestamp: raw.timestamp,
  chat: raw.chat,
  sender: raw.senderJid,
  played: false, // true Kalau Mau Tandai Voice Note Sebagai "Sudah Diputar"
});
```

---

## Download Media

Gunakan `conn.downloadMedia()` untuk mengunduh gambar, video, stiker, audio, atau dokumen dari pesan yang dibalas.

```javascript
const res = await conn.downloadMedia(
  { id: m.quotedId },
  "./downloads", // Folder tujuan
);

console.log(res.filePath);   // Lokasi file
console.log(res.fileName);   // Nama file
console.log(res.mediaType);  // image / video / audio / document / sticker
console.log(res.ext);        // Ekstensi file
```

> **Tips:** Kalau kamu menentukan folder tujuan sendiri, gunakan **absolute path** (misalnya lewat `path.resolve("./downloads")`) alih-alih path relatif seperti `"./downloads"` langsung. Ini untuk menghindari file tersimpan di lokasi yang tidak kamu duga, karena proses Go berjalan dengan working directory sendiri.

```javascript
import path from "path";
import fs from "fs";

const DOWNLOAD_DIR = path.resolve("./downloads");

if (!fs.existsSync(DOWNLOAD_DIR)) {
  fs.mkdirSync(DOWNLOAD_DIR, { recursive: true });
}

const res = await conn.downloadMedia({ id: m.quotedId }, DOWNLOAD_DIR);
```

---

## Ubah Audio Jadi Voice Note (`tovn`)

Kirimkan pesan `.tovn` sambil membalas (reply) pesan audio, dan bot akan otomatis mengonversinya jadi Voice Note (PTT) menggunakan FFmpeg (codec Opus, mono, bitrate 32kbps).

Contoh alur pemakaian di dalam handler pesan:

```javascript
if (["tovn"].includes(command)) {
  if (!m.quotedId) {
    return conn.sendMessage(jid, { text: "Balas pesan audio dulu ya!" });
  }

  // Library akan menangani proses download -> convert -> kirim -> cleanup secara otomatis
}
```

File sementara (input audio & hasil `.ogg`) otomatis dibersihkan setelah voice note terkirim.

---

## Kompres Gambar

Balas gambar dengan perintah  `.kompres` untuk mengompres ukurannya (kualitas JPEG 70, dioptimalkan dengan MozJPEG).

```javascript
if (["kompres"].includes(command)) {
  if (!m.quotedId) {
    return conn.sendMessage(jid, { text: "Balas gambar dulu ya!" });
  }

  // Library akan mengirim balik gambar hasil kompresi
  // beserta perbandingan ukuran awal & ukuran baru
}
```

Contoh caption hasil yang dikirim otomatis:

```
Berhasil Dikompres!

Ukuran Awal: 1024.5 KB
Ukuran Baru: 245.7 KB
```

---

## Buat & Ubah Stiker

### Gambar/GIF/Video → Stiker

Balas gambar, GIF, atau video pendek dengan perintah `.sticker`:

| Media Input       | Hasil                  |
| ----------------- | ---------------------- |
| JPG / PNG / WebP   | Stiker WebP statis     |
| GIF / MP4 / MOV / WEBM / MKV | Stiker WebP animasi |

Batasan untuk stiker animasi:
- Durasi maksimal **10 detik**
- Ukuran file maksimal **~0.99 MB**
- Resolusi **512x512**
- **15 FPS**
- Audio otomatis dihapus

### Stiker: Gambar/Video (`tomedia`)

Balas stiker dengan perintah `.tomedia` untuk mengubahnya kembali:

| Jenis Stiker    | Hasil        |
| --------------- | ------------ |
| Stiker Statis   | Gambar JPEG  |
| Stiker Animasi  | Video MP4    |

Semua proses (deteksi jenis media, konversi, sampai pengiriman) ditangani otomatis oleh library — kamu cukup memanggil perintahnya.

---

## Daftar Event

| Event | Isi Data |
| :--- | :--- |
| `connection.update` | Status koneksi (`open`, `isLoggedIn`, `botJid`, dll) |
| `messages.upsert` | Pesan masuk |
| `pairing_code` | Kode pairing 8 digit (untuk login via nomor HP) |
| `error` | Error internal dari engine |

---

## Menyalakan & Menghentikan Bot

Sebelum melakukan aksi apa pun (termasuk `requestPairingCode`), proses Go engine harus dinyalakan lebih dulu:

```javascript
conn.start();
```

Untuk mematikan bot secara terprogram tanpa merusak sesi:

```javascript
await conn.stop();
```

Bot juga sudah menangani sinyal `Ctrl+C` (`SIGINT`/`SIGTERM`) secara otomatis, jadi sesi kamu tetap aman meski proses dihentikan tiba-tiba.

---

## Troubleshooting Umum

**Q: Kenapa file yang di-download tidak muncul di folder yang saya harapkan?**
A: Pastikan kamu memakai absolute path (`path.resolve(...)`) saat menentukan folder tujuan download, bukan path relatif langsung.

**Q: Kenapa bot gagal menambahkan anggota ke grup (error 403)?**
A: Kemungkinan besar target mengaktifkan privasi "siapa yang bisa menambahkanku ke grup". Cek `res.inviteRequired` dan kirimkan `res.inviteLink` sebagai gantinya.

**Q: Koneksi WhatsApp saya sering putus, apa perlu scan ulang?**
A: Tidak. Engine akan otomatis mencoba menyambung ulang di latar belakang. Kamu bisa memantau prosesnya lewat event `connection.update`.

**Q: Muncul error `"Go process belum berjalan."` saat memanggil `requestPairingCode()` (atau method lain).**
A: `makeWASocket()` hanya menyiapkan instance-nya saja, belum menyalakan proses Go engine-nya. Kamu wajib memanggil `conn.start()` secara manual sebelum menggunakan method apa pun (`requestPairingCode`, `sendMessage`, dll). Perintah yang dipanggil setelah `start()` akan otomatis antre sampai proses Go-nya siap, jadi tidak perlu menambahkan delay/`setTimeout` manual.

**Q: Apa bedanya `sendPresence()` dan `sendChatPresence()`?**
A: `sendPresence()` mengatur status online/offline bot secara global (`available`/`unavailable`). `sendChatPresence()` mengatur indikator "sedang mengetik"/"sedang merekam" di chat tertentu (`composing`/`recording`/`paused`), dan hanya terlihat oleh lawan chat di percakapan itu.

## Credit
- github.com/Lenwyy
- github.com/kingard888

## License

- Proyek ini dilisensikan di bawah [Mozilla Public License 2.0](LICENSE). 
- Proyek ini dibungkus menggunakan dan memanfaatkan library [whatsmeow](https://github.com/tulir/whatsmeow) yang berlisensi MPL-2.0.