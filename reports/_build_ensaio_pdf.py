#!/usr/bin/env python3
"""Render the 10-minute load report to a print HTML page."""

from pathlib import Path

OUT = Path(__file__).with_name("ensaio-reset-10m.html")

MINUTES = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "10"]
RATE = [1997.9, 1999.9, 2000.2, 1999.9, 1999.9, 2000.3, 2000.0, 2000.0, 2000.1, 1998.7]
CLOCK = ["15:19", "15:20", "15:21", "15:22", "15:23", "15:24", "15:25", "15:26", "15:27", "15:28", "15:29"]
MAX_BATCH_MS = [112, 288, 98, 429, 182, 153, 344, 245, 211, 210, 153]
QUEUE_DEPTH = [22, 260, 3, 102, 5, 100, 354, 81, 119, 7, 7]


def num(value, digits=0):
    text = f"{value:,.{digits}f}"
    return text.replace(",", "X").replace(".", ",").replace("X", ".")


def svg_line(categories, values, y_max, color, ref=None, ref_label=None, width=680, height=228):
    left, right, top, bottom = 44, 10, 16, 26
    plot_w = width - left - right
    plot_h = height - top - bottom

    def x_at(index):
        if len(categories) == 1:
            return left + plot_w / 2
        return left + index * plot_w / (len(categories) - 1)

    def y_at(value):
        return top + plot_h * (1 - value / y_max)

    parts = [
        f'<svg viewBox="0 0 {width} {height}" xmlns="http://www.w3.org/2000/svg">'
    ]
    for step in range(5):
        tick = y_max * step / 4
        y = y_at(tick)
        parts.append(
            f'<line x1="{left}" y1="{y:.1f}" x2="{width - right}" y2="{y:.1f}" stroke="#e7e5e4"/>'
        )
        parts.append(
            f'<text x="{left - 6}" y="{y + 3:.1f}" text-anchor="end" font-size="9" fill="#78716c">{num(tick)}</text>'
        )
    if ref is not None:
        y = y_at(ref)
        parts.append(
            f'<line x1="{left}" y1="{y:.1f}" x2="{width - right}" y2="{y:.1f}" stroke="#92400e" stroke-dasharray="4 3"/>'
        )
        if ref_label:
            parts.append(
                f'<text x="{width - right - 2}" y="{max(y - 4, 10):.1f}" text-anchor="end" font-size="9" fill="#92400e">{ref_label}</text>'
            )
    points = " ".join(f"{x_at(i):.1f},{y_at(v):.1f}" for i, v in enumerate(values))
    parts.append(f'<polyline fill="none" stroke="{color}" stroke-width="2" points="{points}"/>')
    for index, value in enumerate(values):
        parts.append(
            f'<circle cx="{x_at(index):.1f}" cy="{y_at(value):.1f}" r="2.4" fill="{color}"/>'
        )
    for index, label in enumerate(categories):
        parts.append(
            f'<text x="{x_at(index):.1f}" y="{height - 6}" text-anchor="middle" font-size="9" fill="#57534e">{label}</text>'
        )
    parts.append("</svg>")
    return "\n".join(parts)


def svg_bars(values=(13.2, 33.4, 132.1, 620.3), y_max=700):
    labels = ["Mediana", "p95", "p99", "Máximo"]
    width, height = 680, 250
    left, right, top, bottom = 44, 10, 18, 28
    plot_w = width - left - right
    plot_h = height - top - bottom
    count = len(values)
    gap = 36
    bar_w = (plot_w - gap * (count + 1)) / count

    def y_at(value):
        return top + plot_h * (1 - value / y_max)

    parts = [f'<svg viewBox="0 0 {width} {height}" xmlns="http://www.w3.org/2000/svg">']
    for step in range(5):
        tick = y_max * step / 4
        y = y_at(tick)
        parts.append(
            f'<line x1="{left}" y1="{y:.1f}" x2="{width - right}" y2="{y:.1f}" stroke="#e7e5e4"/>'
        )
        parts.append(
            f'<text x="{left - 6}" y="{y + 3:.1f}" text-anchor="end" font-size="9" fill="#78716c">{num(tick)}</text>'
        )
    ref_y = y_at(300)
    parts.append(
        f'<line x1="{left}" y1="{ref_y:.1f}" x2="{width - right}" y2="{ref_y:.1f}" stroke="#92400e" stroke-dasharray="4 3"/>'
    )
    parts.append(
        f'<text x="{width - right - 2}" y="{ref_y - 4:.1f}" text-anchor="end" font-size="9" fill="#92400e">meta p99 · 300 ms</text>'
    )
    baseline = top + plot_h
    for index, value in enumerate(values):
        x = left + gap + index * (bar_w + gap)
        y = y_at(value)
        parts.append(
            f'<rect x="{x:.1f}" y="{y:.1f}" width="{bar_w:.1f}" height="{baseline - y:.1f}" fill="#1e3a5f"/>'
        )
        parts.append(
            f'<text x="{x + bar_w / 2:.1f}" y="{y - 6:.1f}" text-anchor="middle" font-size="11" fill="#1c1917">{num(value, 1)} ms</text>'
        )
        parts.append(
            f'<text x="{x + bar_w / 2:.1f}" y="{height - 8}" text-anchor="middle" font-size="10" fill="#57534e">{labels[index]}</text>'
        )
    parts.append("</svg>")
    return "\n".join(parts)


def flow():
    return """
<svg viewBox="0 0 680 300" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <marker id="a" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
      <path d="M0,0 L8,4 L0,8 Z" fill="#1e3a5f"/>
    </marker>
    <marker id="m" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
      <path d="M0,0 L8,4 L0,8 Z" fill="#78716c"/>
    </marker>
  </defs>
  <text x="0" y="14" font-size="12" font-weight="600" fill="#1c1917">Autorização local — percorrida pelo ensaio</text>
  <g font-size="11" font-weight="600" fill="#1c1917">
    <g>
      <rect x="0" y="24" width="100" height="52" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
      <text x="50" y="44" text-anchor="middle">k6</text>
      <text x="50" y="60" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">2.000 req/s</text>
      <line x1="102" y1="50" x2="112" y2="50" stroke="#1e3a5f" marker-end="url(#a)"/>
    </g>
    <g>
      <rect x="114" y="24" width="100" height="52" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
      <text x="164" y="44" text-anchor="middle">Duas APIs</text>
      <text x="164" y="60" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">:8080 e :8081</text>
      <line x1="216" y1="50" x2="226" y2="50" stroke="#1e3a5f" marker-end="url(#a)"/>
    </g>
    <g>
      <rect x="228" y="24" width="100" height="52" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
      <text x="278" y="44" text-anchor="middle">Fila do lote</text>
      <text x="278" y="60" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">até 64 · 15 ms</text>
      <line x1="330" y1="50" x2="340" y2="50" stroke="#1e3a5f" marker-end="url(#a)"/>
    </g>
    <g>
      <rect x="342" y="24" width="100" height="52" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
      <text x="392" y="44" text-anchor="middle">Lock</text>
      <text x="392" y="60" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">um escritor</text>
      <line x1="444" y1="50" x2="454" y2="50" stroke="#1e3a5f" marker-end="url(#a)"/>
    </g>
    <g>
      <rect x="456" y="24" width="100" height="52" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
      <text x="506" y="44" text-anchor="middle">COMMIT</text>
      <text x="506" y="60" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">operação + partidas</text>
      <line x1="558" y1="50" x2="568" y2="50" stroke="#1e3a5f" marker-end="url(#a)"/>
    </g>
    <g>
      <rect x="570" y="24" width="100" height="52" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
      <text x="620" y="44" text-anchor="middle">HTTP 200</text>
      <text x="620" y="60" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">depois do commit</text>
    </g>
  </g>

  <text x="0" y="112" font-size="12" font-weight="600" fill="#57534e">Consulta — presente na API, sem tráfego neste k6</text>
  <g font-size="11" font-weight="600" fill="#1c1917">
    <rect x="0" y="122" width="140" height="48" rx="4" fill="#fff" stroke="#d6d3d1"/>
    <text x="70" y="140" text-anchor="middle">GET /balance</text>
    <text x="70" y="156" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">conta positiva</text>
    <line x1="142" y1="146" x2="154" y2="146" stroke="#78716c" marker-end="url(#m)"/>
    <rect x="156" y="122" width="150" height="48" rx="4" fill="#fff" stroke="#d6d3d1"/>
    <text x="231" y="140" text-anchor="middle">GET /statement</text>
    <text x="231" y="156" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">página por sequência</text>
    <line x1="308" y1="146" x2="320" y2="146" stroke="#78716c" marker-end="url(#m)"/>
    <rect x="322" y="122" width="140" height="48" rx="4" fill="#fff" stroke="#d6d3d1"/>
    <text x="392" y="140" text-anchor="middle">Última partida</text>
    <text x="392" y="156" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">saldo vigente</text>
  </g>

  <text x="0" y="206" font-size="12" font-weight="600" fill="#57534e">Destino externo — presente no processo, outbox ficou em zero</text>
  <g font-size="11" font-weight="600" fill="#1c1917">
    <rect x="0" y="216" width="120" height="48" rx="4" fill="#fff" stroke="#d6d3d1"/>
    <text x="60" y="234" text-anchor="middle">Outbox</text>
    <text x="60" y="250" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">no mesmo COMMIT</text>
    <line x1="122" y1="240" x2="134" y2="240" stroke="#78716c" marker-end="url(#m)"/>
    <rect x="136" y="216" width="130" height="48" rx="4" fill="#fff" stroke="#d6d3d1"/>
    <text x="201" y="234" text-anchor="middle">Despachante</text>
    <text x="201" y="250" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">a cada 20 ms</text>
    <line x1="268" y1="240" x2="280" y2="240" stroke="#78716c" marker-end="url(#m)"/>
    <rect x="282" y="216" width="130" height="48" rx="4" fill="#fff" stroke="#d6d3d1"/>
    <text x="347" y="234" text-anchor="middle">Simulador</text>
    <text x="347" y="250" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">pix_external</text>
    <line x1="414" y1="240" x2="426" y2="240" stroke="#78716c" marker-end="url(#m)"/>
    <rect x="428" y="216" width="150" height="48" rx="4" fill="#fff" stroke="#d6d3d1"/>
    <text x="503" y="234" text-anchor="middle">Liquidação</text>
    <text x="503" y="250" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">SETTLED ou estorno</text>
  </g>
  <path d="M506 76 V 196 H 60 V 216" fill="none" stroke="#a8a29e" stroke-dasharray="4 3" marker-end="url(#m)"/>
  <text x="514" y="150" font-size="9" fill="#78716c">destino externo</text>
</svg>
"""


def topology():
    return """
<svg viewBox="0 0 680 150" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <marker id="t" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
      <path d="M0,0 L8,4 L0,8 Z" fill="#1e3a5f"/>
    </marker>
  </defs>
  <rect x="0" y="40" width="110" height="52" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
  <text x="55" y="60" text-anchor="middle" font-size="11" font-weight="600">API :8080</text>
  <text x="55" y="76" text-anchor="middle" font-size="9" fill="#57534e">lote em memória</text>
  <rect x="0" y="100" width="110" height="40" rx="4" fill="#fff" stroke="#d6d3d1"/>
  <text x="55" y="124" text-anchor="middle" font-size="11" font-weight="600">API :8081</text>
  <line x1="112" y1="66" x2="168" y2="66" stroke="#1e3a5f" marker-end="url(#t)"/>
  <line x1="112" y1="120" x2="168" y2="78" stroke="#1e3a5f" marker-end="url(#t)"/>
  <rect x="170" y="44" width="150" height="56" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
  <text x="245" y="66" text-anchor="middle" font-size="11" font-weight="600">Primário</text>
  <text x="245" y="82" text-anchor="middle" font-size="9" fill="#57534e">COMMIT da conta 1</text>
  <line x1="322" y1="72" x2="378" y2="72" stroke="#78716c" stroke-dasharray="4 3"/>
  <rect x="380" y="44" width="150" height="56" rx="4" fill="#fff" stroke="#d6d3d1"/>
  <text x="455" y="66" text-anchor="middle" font-size="11" font-weight="600">Réplica</text>
  <text x="455" y="82" text-anchor="middle" font-size="9" fill="#57534e">só lê o WAL</text>
  <text x="0" y="16" font-size="11" font-weight="600" fill="#1c1917">O lote das duas APIs grava no mesmo primário</text>
  <text x="380" y="122" font-size="9" fill="#78716c">não processa POST /transfers</text>
</svg>
"""


HTML = """<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<title>Ensaio de 2.000 req/s em 10 minutos</title>
<style>
  @page {{ size: A4; margin: 14mm 14mm 16mm; }}
  * {{ box-sizing: border-box; }}
  html, body {{ margin: 0; padding: 0; }}
  body {{
    font-family: "Helvetica Neue", Helvetica, Arial, sans-serif;
    color: #1c1917;
    font-size: 11px;
    line-height: 1.45;
    -webkit-print-color-adjust: exact;
    print-color-adjust: exact;
  }}
  h1 {{ font-size: 22px; line-height: 1.2; margin: 0 0 6px; font-weight: 650; letter-spacing: -0.02em; }}
  h2 {{ font-size: 14px; margin: 18px 0 6px; font-weight: 650; letter-spacing: -0.01em; break-after: avoid; page-break-after: avoid; }}
  p {{ margin: 0 0 8px; }}
  .muted {{ color: #57534e; }}
  .small {{ font-size: 9.5px; color: #78716c; margin-top: 2px; }}
  .rule {{ height: 2px; background: #1c1917; margin: 0 0 12px; }}
  .kpis {{ display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; margin: 12px 0 4px; }}
  .kpi {{ border: 1px solid #e7e5e4; padding: 8px 10px; }}
  .kpi b {{ display: block; font-size: 18px; letter-spacing: -0.02em; font-weight: 650; }}
  .kpi span {{ color: #57534e; font-size: 10px; }}
  .kpi.bad b {{ color: #991b1b; }}
  .kpi.ok b {{ color: #166534; }}
  .note {{
    border-left: 3px solid #92400e;
    background: #fffbeb;
    padding: 8px 10px;
    margin: 10px 0 4px;
  }}
  .note strong {{ display: block; margin-bottom: 2px; }}
  .info {{
    border-left: 3px solid #1e3a5f;
    background: #f8fafc;
    padding: 8px 10px;
    margin-top: 12px;
  }}
  figure {{ margin: 4px 0 10px; page-break-inside: avoid; }}
  figcaption {{ font-size: 9.5px; color: #78716c; margin-top: 2px; }}
  svg {{ width: 100%; height: auto; display: block; }}
  table {{ width: 100%; border-collapse: collapse; margin: 4px 0 8px; page-break-inside: avoid; }}
  th {{ text-align: left; font-size: 10px; font-weight: 650; border-bottom: 1.5px solid #1c1917; padding: 5px 6px; }}
  td {{ border-bottom: 1px solid #e7e5e4; padding: 4px 6px; vertical-align: top; }}
  td.num, th.num {{ text-align: right; font-variant-numeric: tabular-nums; }}
  .pass {{ color: #166534; font-weight: 650; }}
  .fail {{ color: #991b1b; font-weight: 650; }}
  .keep {{ break-inside: avoid; page-break-inside: avoid; }}
  header .meta {{ color: #57534e; font-size: 11px; }}
</style>
</head>
<body>
<header>
  <div class="rule"></div>
  <h1>Ensaio de 2.000 req/s em 10 minutos</h1>
  <p class="meta">Conta quente 1, modo misto, duas APIs e PostgreSQL 16. Volume recriado, 1.000.000 de clientes semeados, depois k6 com chegada aberta. 29 de setembro de 2026, 15:19–15:29, horário de Brasília.</p>
</header>

<div class="note">
  <strong>A meta de vazão passou no p99 e falhou nos erros.</strong>
  O p99 ficou em 132 ms, abaixo de 300 ms, e nenhuma iteração foi descartada. Três das 1.200.013 requisições não voltaram HTTP 200. O k6 encerrou com código 99 por causa desses três casos. Nenhuma delas gerou operação no banco.
</div>

<div class="kpis">
  <div class="kpi ok"><b>132 ms</b><span>p99 HTTP ponta a ponta</span></div>
  <div class="kpi"><b>1.999,9/s</b><span>Taxa concluída</span></div>
  <div class="kpi ok"><b>1.200.010</b><span>Aprovações gravadas</span></div>
  <div class="kpi bad"><b>3</b><span>Respostas fora de HTTP 200</span></div>
</div>

<section class="keep">
  <h2>Latência HTTP do ensaio</h2>
  <p class="muted">Mediana, p95, p99 e máximo de http_req_duration. A linha tracejada é a meta de p99 abaixo de 300 ms. Eixo vertical em milissegundos.</p>
  <figure>
    {bars}
    <figcaption>Fonte: reports/load-reset-10m.json · k6 · 10m00s · localhost:8080 e localhost:8081</figcaption>
  </figure>
</section>

<section class="keep">
  <h2>Vazão sustentada</h2>
  <p class="muted">Requisições concluídas por segundo em cada minuto do relógio do k6. A linha tracejada é a taxa pedida, 2.000 req/s. Eixo vertical: req/s, começando em zero.</p>
  <figure>
    {rate}
    <figcaption>Eixo horizontal: minuto do ensaio, de 1 a 10. Fonte: progresso do k6, 15:19–15:29.</figcaption>
  </figure>
</section>

<section class="keep">
  <h2>Maior commit de lote</h2>
  <p class="muted">Máximo de max_batch_ms entre as duas APIs, em cada minuto. Eixo horizontal: hora local. Eixo vertical: milissegundos. A linha tracejada é 300 ms.</p>
  <figure>
    {batch}
    <figcaption>15:19 e 15:29 são minutos parciais. Fonte: log queue stats.</figcaption>
  </figure>
</section>

<section class="keep">
  <h2>Profundidade da fila</h2>
  <p class="muted">Maior depth no intervalo de 5 s. Cada fila cabe 4.096 pedidos. Eixo horizontal: hora local. Eixo vertical: pedidos.</p>
  <figure>
    {depth}
    <figcaption>Fonte: log queue stats das duas APIs.</figcaption>
  </figure>
</section>

<section class="keep">
  <h2>Critério do ensaio saudável</h2>
  <table>
    <thead>
      <tr><th>Critério</th><th class="num">Meta</th><th class="num">Medido</th><th>Resultado</th></tr>
    </thead>
    <tbody>
      <tr><td>p99 HTTP</td><td class="num">&lt; 300 ms</td><td class="num">132,1 ms</td><td class="pass">Passou</td></tr>
      <tr><td>Erros técnicos</td><td class="num">0</td><td class="num">3</td><td class="fail">Falhou</td></tr>
      <tr><td>Iterações descartadas</td><td class="num">0</td><td class="num">0</td><td class="pass">Passou</td></tr>
      <tr><td>Fila cheia</td><td class="num">0</td><td class="num">0</td><td class="pass">Passou</td></tr>
      <tr><td>Timeout de lote</td><td class="num">0</td><td class="num">0</td><td class="pass">Passou</td></tr>
    </tbody>
  </table>
  <p>As três falhas entraram no contador de outro status do script: nem HTTP 200 nem HTTP 503. Os logs das duas APIs fecharam com batch_fail=0, queue_full=0 e timeout_waiting=0. O banco tem 1.200.010 operações, o mesmo número de HTTP 200.</p>
</section>

<section class="keep">
  <h2>O que ficou gravado</h2>
  <table>
    <thead><tr><th>Conferência no PostgreSQL</th><th class="num">Valor</th></tr></thead>
    <tbody>
      <tr><td>Contas (1.000.000 clientes + 32 controles)</td><td class="num">1.000.032</td></tr>
      <tr><td>Operações, todas APPROVED</td><td class="num">1.200.010</td></tr>
      <tr><td>DECLINED</td><td class="num">0</td></tr>
      <tr><td>Partidas (duas por aprovação)</td><td class="num">2.400.020</td></tr>
      <tr><td>Partidas da conta 1</td><td class="num">1.200.010</td></tr>
      <tr><td>Última sequência da conta 1</td><td class="num">1.200.010</td></tr>
      <tr><td>Saldo final da conta 1</td><td class="num">1.000.000.000.000</td></tr>
      <tr><td>Menor saldo visto na conta 1</td><td class="num">999.999.999.935</td></tr>
      <tr><td>Prefixos negativos na conta 1</td><td class="num">0</td></tr>
      <tr><td>Eventos na outbox</td><td class="num">0</td></tr>
    </tbody>
  </table>
  <p class="small">Consultas SQL logo após o k6, banco pix. O saldo está em centavos. Cada transferência movia 1 centavo, metade entrando e metade saindo da conta 1, então o saldo final voltou ao seed. A linha de accounts da conta 1 continua com last_sequence 0: o caminho do lote grava a sequência só em entries.</p>
</section>

<section class="keep">
  <h2>Fluxo do autorizador</h2>
  <p>O ensaio percorreu só a faixa de cima: transferência local, conta 1 contra um par, valor de 1 centavo. As faixas de baixo existem no processo e ficaram paradas nesta execução. O traço tracejado sai do COMMIT quando o destino é externo.</p>
  <figure>
    {flow}
    <figcaption>Lote 64, janela de 15 ms, lock consultivo e prazo de commit de 1,2 s. fsync e synchronous_commit ligados.</figcaption>
  </figure>
</section>

<section class="keep">
  <h2>Camadas do processo</h2>
  <table>
    <thead><tr><th>Peça</th><th>O que faz neste desenho</th></tr></thead>
    <tbody>
      <tr><td>k6 · scripts/load.js</td><td>Chegada aberta. Alterna crédito e débito da conta 1 e reparte as chamadas entre as duas portas.</td></tr>
      <tr><td>cmd/pix</td><td>Sobe a API, aplica migração e, com APP_MODE=simulator, vira o destinatário externo.</td></tr>
      <tr><td>internal/httpapi</td><td>Autentica a chave, decodifica o JSON e espera a resposta do lote. O despachante da outbox mora aqui.</td></tr>
      <tr><td>internal/ledger</td><td>Decide cada débito em ordem, com saldo intermediário, sem I/O.</td></tr>
      <tr><td>internal/postgres</td><td>Fila em memória, lock consultivo, COMMIT de operations e entries.</td></tr>
      <tr><td>PostgreSQL 16</td><td>Autoridade do saldo. fsync e synchronous_commit ligados. shared_buffers de 512 MB.</td></tr>
      <tr><td>Simulador</td><td>Banco pix_external. Um efeito por ID. O prefixo reject: no destinatário vira rejeição definitiva.</td></tr>
    </tbody>
  </table>
</section>

<section class="keep">
  <h2>Escala: o que este ensaio mostra</h2>
  <p>A recepção HTTP já corre ao lado do banco: cada POST entra na fila e espera o canal de resposta. O que não paraleliza é o próximo COMMIT. Um único goroutine por API drena a fila; enquanto o lote está no fsync, pedidos novos só ocupam o canal (4.096 lugares). Neste ensaio a profundidade chegou a 354, queue_full ficou em 0 e o p99 acompanhou os commits lentos (até 429 ms), não a CPU da API.</p>
  <p>Escalar horizontalmente aqui é partir o razão por conta, não multiplicar o processo HTTP. Mais API na frente do mesmo primário não aumenta a vazão da conta quente. O ensaio já rodou com duas instâncias: as duas esperam o lock 937513 e um COMMIT com fsync.</p>
</section>

<section class="keep">
  <h2>Duas APIs, um primário, réplica</h2>
  <p class="muted">O lote não escolhe uma réplica. Toda autorização grava no primário.</p>
  <figure>
    {topology}
    <figcaption>As duas APIs deste Compose apontam para o mesmo DATABASE_URL. Réplica de leitura não existia neste ensaio; o desenho mostra o papel dela se existisse.</figcaption>
  </figure>
  <table>
    <thead><tr><th>Peça</th><th>O que faz</th><th>O que não faz</th></tr></thead>
    <tbody>
      <tr><td>API :8080 e :8081</td><td>Aceitam HTTP. Se uma cai, a outra segue. As duas mandam SQL para o mesmo Postgres.</td><td>Não dobram a vazão da conta 1. Os lotes entram no mesmo lock 937513.</td></tr>
      <tr><td>Primário PostgreSQL</td><td>Único lugar que faz COMMIT de operations e entries. O lote sempre grava aqui.</td><td>Não despacha “para uma réplica”. Não há segundo escritor da conta 1.</td></tr>
      <tr><td>Réplica de leitura</td><td>Copia o WAL já commitado. Serve GET se o atraso for aceitável. Se o primário morre, pode ser promovida (failover).</td><td>Não processa lote. Não disputa COMMIT. Não aumenta 2.000 req/s na conta quente.</td></tr>
    </tbody>
  </table>
  <p>Réplica não é segundo primário. Dois bancos gravando a conta 1 ao mesmo tempo recriam a disputa que o lote existe para evitar. Exemplo: saldo 100, dois créditos de 1. Um escritor junta os dois, decide 101 e 102 e faz um fsync. Dois primários leem os dois “última partida = 100”, tentam a próxima sequência, dois fsync, cauda alta. Foi o que aconteceu quando as duas APIs reescreviam a linha da conta 1; o lock único deixou um lote no disco por vez.</p>
</section>

<section class="keep">
  <h2>O que seria um shard</h2>
  <p>Shard é um pedaço autônomo do razão: um conjunto de contas, o Postgres que as guarda e o escritor que avança o saldo delas. Não é réplica da API. A API continua stateless. Conta 1 mora num pedaço; outras contas em outro. Cada shard tem o próprio accounts, operations e entries daquelas contas, e um COMMIT próprio. Dois shards não atualizam a mesma linha.</p>
  <p>O protótipo já usa a ideia em miniatura: as 32 contas técnicas de saída, escolhidas por hash estável da origem, para o Pix externo não brigar por uma linha só. O próximo passo seria o mesmo critério para clientes: hash(account_id) → qual banco.</p>
  <p>Transferência <strong>dentro</strong> do shard é o fluxo deste ensaio: lote, duas partidas, COMMIT, depois HTTP 200. Transferência <strong>entre</strong> shards não cabe nesse COMMIT: débito num disco e crédito noutro, como o destino externo já faz com outbox e PENDING. A conta que recebe 2.000 req/s não se parte: ela inteira vai para um shard só. Espelhar essa conta em dois primários recria a disputa acima.</p>
</section>

<div class="note">
  <strong>Neste ensaio de 10 minutos, shard não melhoraria o resultado.</strong>
  Os 2.000 req/s tocam a conta 1. Um shard por conta mandaria tudo ao mesmo pedaço. O COMMIT, o fsync e o escritor único continuariam iguais. O p99 de 132 ms não cairia. Shard ajuda o total quando o tráfego se espalha por contas que não se cruzam — e só depois de o lock deixar de ser um para o banco inteiro (hoje o 937513 serializa até contas frias).
</div>

<section>
  <h2>Se a meta for 4.000 req/s</h2>
  <p class="muted">Não se infere do p99 de 132 ms. Medir 2.500, 3.000 e 4.000 no mesmo cenário misto. Manter fsync; desligar não é “dar suporte”.</p>
  <table>
    <thead><tr><th>Cenário</th><th>O que falta</th></tr></thead>
    <tbody>
      <tr>
        <td>Conta 1 a 4.000/s</td>
        <td>Dobro deste ensaio no mesmo caderno. Com lote 64 seriam ~63 COMMIT/s (cerca de 16 ms por gravação). O lote já chegou a 429 ms: sem mais pedidos por fsync, um escritor só da conta 1 e WAL mais rápido, a fila de 4.096 enche e volta erro técnico. Réplica e segundo primário da conta 1 não resolvem. HTTP pode repetir; quem commita a conta 1 deve ser um.</td>
      </tr>
      <tr>
        <td>4.000 no total, contas que quase não se cruzam</td>
        <td>O teto atual é o lock 937513 (um COMMIT no banco inteiro). Primeiro travar só as contas do lote; depois shards. Cada fatia aguenta o próprio lote. Quem sozinha chegar a 4.000 continua um shard só dela.</td>
      </tr>
    </tbody>
  </table>
  <p>O contrato não muda em nenhum desenho: APPROVED depois do COMMIT, um efeito por ID, duas partidas por aprovação, saldo nunca negativo em todo prefixo, erro técnico nunca vira DECLINED.</p>
</section>

<div class="info">
  <strong>Limite deste relatório.</strong>
  Estes números são o ensaio misto de 10 minutos depois do reset. Consulta de saldo, extrato, pagamento externo, contas frias, fan-in isolado, fan-out isolado e o auditor independente não rodaram nesta execução. A discussão de escala e de 4.000 req/s não foi medida.
</div>
</body>
</html>
"""


def main():
    html = HTML.format(
        bars=svg_bars(),
        rate=svg_line(MINUTES, RATE, 2400, "#1e3a5f", ref=2000, ref_label="2.000 req/s"),
        batch=svg_line(CLOCK, MAX_BATCH_MS, 500, "#92400e", ref=300, ref_label="300 ms"),
        depth=svg_line(CLOCK, QUEUE_DEPTH, 400, "#44403c"),
        flow=flow(),
        topology=topology(),
    )
    OUT.write_text(html, encoding="utf-8")
    print(OUT)


if __name__ == "__main__":
    main()
