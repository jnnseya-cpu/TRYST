#!/usr/bin/env python3
"""Build docs/spec/TRYST_Spec_v1.3.pdf: python3 scripts/build_spec_pdf.py out.html, then print with headless Chromium (see README)."""
import markdown, re, sys
from pathlib import Path
root=Path.cwd(); logo=(root/'assets/brand/tryst-logo.png').as_uri()
parts=[f'''<section class="cover"><img src="{logo}" alt="TRYST logo"><div class="t1">Product &amp; Technical Specification</div>
<div class="t2">Version 1.2 — Baseline · 4 October 2026 · Confidential</div><div class="t3">Private chemistry. Intelligent discretion.</div></section>''']
for f in sorted(Path('docs/spec').glob('0*.md')):
    md=f.read_text()
    md=re.sub(r'<p align="center"><img[^>]*></p>\n','',md)
    md=md.replace('../../assets/brand/tryst-logo.png', logo)
    md=re.sub(r'\]\((0\d_[^)#]+\.md)#([^)]+)\)', lambda m: f'](#{m.group(2)})', md)
    md=re.sub(r'\]\((0\d_[^)#]+\.md)\)', lambda m: f'](#doc-{m.group(1)[:2]})', md)
    L=md.split('\n'); o=[]
    for i,l in enumerate(L):
        if re.match(r'^(\s*)([-*]|\d+\.) ',l) and o and o[-1].strip() and not re.match(r'^(\s*)([-*]|\d+\.) ',o[-1]) and not o[-1].startswith('|'): o.append('')
        o.append(l)
    md='\n'.join(o)
    html=markdown.markdown(md, extensions=['tables','fenced_code','toc'])
    parts.append(f'<section class="doc" id="doc-{f.name[:2]}">{html}</section>')
css="""@page{size:A4;margin:14mm 13mm}@page:first{margin:0}
*{-webkit-print-color-adjust:exact;print-color-adjust:exact}
body{font-family:'DejaVu Sans',Arial,sans-serif;font-size:8.6pt;line-height:1.35;color:#1d1416}
.cover{background:#0B0607;height:295mm;overflow:hidden;page-break-after:always;box-sizing:border-box;margin:0;padding:55mm 13mm 0;text-align:center;color:#FCF2DD}
.cover img{width:120mm}.cover .t1{font-family:'DejaVu Serif',serif;font-size:20pt;color:#CB944F;margin-top:12mm;letter-spacing:.04em}
.cover .t2{font-size:10pt;color:#F5D3AC;margin-top:5mm}.cover .t3{font-family:'DejaVu Serif',serif;font-style:italic;font-size:11pt;color:#CB944F;margin-top:18mm}
.doc{page-break-before:always}.cover+.doc{page-break-before:auto}
h1,h2,h3{font-family:'DejaVu Serif',serif}
h1{font-size:20pt;color:#560001;border-bottom:2px solid #CB944F;padding-bottom:4pt}
h2{font-size:13pt;color:#560001;margin-top:14pt;page-break-after:avoid}
h3{font-size:10.5pt;color:#702A2E;page-break-after:avoid}
table{border-collapse:collapse;width:100%;margin:5pt 0;font-size:7.7pt}
tr{page-break-inside:avoid}th{background:#FCF2DD;color:#3F0000;text-align:left}
th,td{border:1px solid #E3CFB4;padding:2pt 4pt;vertical-align:top}
code,pre{font-family:'DejaVu Sans Mono',monospace;font-size:7.2pt}
pre{background:#FBF6EE;padding:5pt;white-space:pre-wrap;page-break-inside:avoid}
blockquote{border-left:3px solid #CB944F;margin:5pt 0;padding:2pt 9pt;background:#FCF2DD}
a{color:#560001;text-decoration:none}hr{display:none}
.doc img{display:block;margin:4pt 0;background:#0B0607;padding:6pt;border-radius:4pt}"""
Path(sys.argv[1]).write_text(f"<!doctype html><html><head><meta charset=utf-8><title>TRYST Specification v1.3</title><style>{css}</style></head><body>{''.join(parts)}</body></html>")
