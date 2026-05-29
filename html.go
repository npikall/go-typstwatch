package main

const pdfHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Go TypstWatcher</title>
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

const imageHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Go TypstWatcher</title>
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
