# Fonts

Self-hosted fonts. Bukan dimasukkan ke repo karena ukuran besar.

## Inter (UI text)

- Sumber resmi: https://rsms.me/inter/
- Repo GitHub: https://github.com/rsms/inter/releases
- Yang dipakai: file variable `InterVariable.woff2`
- Letakkan di sini sebagai: `web/static/font/InterVariable.woff2`

## JetBrains Mono (numeric, code, SKU)

- Sumber resmi: https://www.jetbrains.com/lp/mono/
- Repo GitHub: https://github.com/JetBrains/JetBrainsMono/releases
- Yang dipakai: file variable `JetBrainsMono-Variable.woff2` (atau konversi ttf to woff2 jika hanya ttf yang tersedia)
- Letakkan di sini sebagai: `web/static/font/JetBrainsMono-Variable.woff2`

## Fallback

Jika file font belum ada, browser akan fallback ke system sans-serif / monospace
(lihat `tailwind.config.js` `fontFamily`). Aplikasi tetap jalan, hanya tampilan
typography yang sedikit beda.

## Catatan

- Lisensi Inter: SIL Open Font License 1.1
- Lisensi JetBrains Mono: SIL Open Font License 1.1
- Self-host alasan: privacy (tidak panggil Google Fonts) + performa (tidak ada DNS lookup eksternal).
