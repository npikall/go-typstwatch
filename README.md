# Go TypstWatcher

Compile Typst Documents and watch them live in the Browser.

```console
go-typstwatch [-port N] [-format pdf|png|svg] [-diagnostic-format human|short]
              [-root DIR] <file.typ>
Usage of go-typstwatch:
  -diagnostic-format string
     typst diagnostic format (human, short) (default "short")
  -format string
     output format passed to typst watch (pdf, png, svg) (default "pdf")
  -port int
     port to listen on (default 42069)
  -root string
     root directory for typst file access (default: input file directory)
```
