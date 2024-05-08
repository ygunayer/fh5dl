# fh5dl
A simple CLI program that allows you to download image based PDFs from flip5html.

This is a rough and quick sketch of a code so it probably doesn't cover all edge cases. Use at your own risk.

## Running
Download the appropriate version for your OS and CPU architecture from the release page, jump into your terminal and run:

```bash
# Windows
$ .\fh5dl.exe https://online.fliphtml5.com/.../...

# Non-Windows
$ ./fh5dl https://online.fliphtml5.com/.../...
```

The program should automatically detect the title of the book and save the PDF file in the current folder with the said title (e.g. "Some Book Title.pdf")

## License
MIT
