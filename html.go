package main

import (
	"html/template"
	"strings"
)

// typstFavicon is the Typst Simple Icons logo (CC0-1.0), brand color #239DAD.
const typstFavicon = "data:image/svg+xml;base64,PHN2ZyByb2xlPSJpbWciIHZpZXdCb3g9IjAgMCAyNCAyNCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj48cGF0aCBmaWxsPSIjMjM5REFEIiBkPSJNMTIuNjU0IDE3Ljg0NmMwIDEuMTE0LjE2IDEuODYxLjQ3OSAyLjI0Mi4zMi4zODEuOTAxLjU3MiAxLjc0My41NzIuODcyIDAgMS45OS0uNDQgMy4zNTYtMS4zMTlsLjg3MSAxLjQ1QzE2LjU0NyAyMi45MzEgMTQuNDQgMjQgMTIuNzg1IDI0Yy0xLjY1NiAwLTIuOTY0LS4zOTUtMy45MjItMS4xODctLjk1OS0uODItMS40MzgtMi4yNTYtMS40MzgtNC4zMDdWNi45ODlINS4yNDZsLS4zNDktMS42MjYgMi41MjgtLjc5MVYyLjQxOEwxMi42NTQgMHY0LjgzNWw1LjE0Mi0uMzk1LS40OCAyLjg1Ny00LjY2Mi0uMTc2djEwLjcyNVoiLz48L3N2Zz4="

type htmlData struct {
	Title   string
	Favicon template.URL
}

var (
	pdfTmpl   = template.Must(template.New("pdf").Parse(rawPdfHTML))
	imageTmpl = template.Must(template.New("image").Parse(rawImageHTML))
)

const rawPdfHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<link rel="icon" type="image/svg+xml" href="{{.Favicon}}">
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
html, body { height: 100%; background: #404040; }
iframe { width: 100%; height: 100%; border: none; display: block; }
</style>
</head>
<body>
<iframe id="viewer" src="/output?t=0"></iframe>
<script>
const es = new EventSource('/events');
es.onmessage = () => {
  const f = document.getElementById('viewer');
  f.src = '/output?t=' + Date.now();
};
</script>
</body>
</html>`

const rawImageHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<link rel="icon" type="image/svg+xml" href="{{.Favicon}}">
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
html, body { background: #404040; }
#pages { display: flex; flex-direction: column; align-items: center; gap: 16px; padding: 16px; }
#pages img { max-width: 100%; box-shadow: 0 2px 8px rgba(0,0,0,0.5); }
</style>
</head>
<body>
<div id="pages"></div>
<script>
async function loadPages() {
  const res = await fetch('/pages');
  const files = await res.json();
  const container = document.getElementById('pages');
  container.innerHTML = '';
  const t = Date.now();
  for (const f of files) {
    const img = document.createElement('img');
    img.src = '/output/' + f + '?t=' + t;
    container.appendChild(img);
  }
}
loadPages();
const es = new EventSource('/events');
es.onmessage = loadPages;
</script>
</body>
</html>`

func buildHTML(format, filename string) string {
	data := htmlData{
		Title:   "TypstWatcher " + filename,
		Favicon: template.URL(typstFavicon),
	}
	tmpl := pdfTmpl
	if format != "pdf" {
		tmpl = imageTmpl
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(err)
	}
	return buf.String()
}
