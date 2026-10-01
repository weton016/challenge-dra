#!/usr/bin/env python3
"""Render the TigerBeetle 10-minute load report to a print HTML page.

Sources: reports/load-tb-*.json, reports/audit-tigerbeetle.json, reports/faults.json
and the per-second progress of the k6 run (rate and busy VUs per minute below).
"""

from pathlib import Path

from _build_ensaio_pdf import HTML as STYLE_SOURCE, num, svg_bars, svg_line

OUT = Path(__file__).with_name("ensaio-tigerbeetle-10m.html")
STYLE = STYLE_SOURCE[STYLE_SOURCE.index("<style>"):STYLE_SOURCE.index("</style>") + 8].replace("{{", "{").replace("}}", "}")

MINUTES = [str(m) for m in range(1, 11)]
RATE = [1998.6, 1999.9, 2000.1, 2000.0, 1998.7, 2001.2, 2000.1, 2000.0, 1999.9, 2001.4]
BUSY_VUS = [27, 106, 155, 48, 149, 145, 131, 1344, 138, 55]

RUNS = [
    ("tb-smoke-30s", "logo após semear 1M, sem janela, cache 512 MiB", 3002, 7523, 3652, "falhou: compactação pós-seed"),
    ("tb-smoke-30s-2", "mesmo cenário, um minuto depois", 51, 0, 0, "passou (30 s)"),
    ("tb-10m-mixed", "10 min, sem janela, cache 512 MiB", 1595, 7245, 1479, "falhou"),
    ("tb-diag-3m", "3 min, diagnóstico por segundo", 660, 96, 0, "travadas de 0,3 a 1,8 s"),
    ("tb-window-smoke-30s", "volume antigo (~5M), janela + 2 GiB, cache frio", 566, 0, 0, "falhou no p99"),
    ("tb-window-smoke-60s", "mesmo volume, 60 s", 2823, 1990, 1614, "falhou: travada de 10 s"),
    ("tb-fresh-smoke-60s", "volume novo, seed 1M, janela + 2 GiB", 94, 0, 0, "passou (60 s)"),
    ("tb-10m-mixed-window", "10 min disparado do host macOS", 148, 4, 0, "falhou: 4 EOF"),
    ("tb-10m-mixed-innet", "10 min, k6 na rede do Compose", 74, 0, 0, "passou"),
]


def runs_table():
    rows = []
    for name, setup, p99, failed, dropped, verdict in RUNS:
        css = "pass" if verdict.startswith("passou") else "fail"
        rows.append(
            f'<tr><td>{name}</td><td>{setup}</td><td class="num">{num(p99)}</td>'
            f'<td class="num">{num(failed)}</td><td class="num">{num(dropped)}</td><td class="{css}">{verdict}</td></tr>'
        )
    return "\n".join(rows)


def box(x, title, sub, y=24, w=100, h=52, strong=True):
    fill, stroke = ("#f5f5f4", "#1e3a5f") if strong else ("#fff", "#d6d3d1")
    return (
        f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="4" fill="{fill}" stroke="{stroke}"/>'
        f'<text x="{x + w / 2}" y="{y + 20}" text-anchor="middle">{title}</text>'
        f'<text x="{x + w / 2}" y="{y + 36}" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">{sub}</text>'
    )


def arrow(x1, x2, y, marker="a", color="#1e3a5f"):
    return f'<line x1="{x1}" y1="{y}" x2="{x2}" y2="{y}" stroke="{color}" marker-end="url(#{marker})"/>'


def flow():
    top = [("k6", "2.000 req/s"), ("Duas APIs", "api e api2"), ("Janela 5 ms", "um pedido ao TB"),
           ("TigerBeetle", "saldo e id no commit"), ("Resultado", "por transferência"), ("HTTP 200", "depois do commit")]
    bottom = [("Cadeia atômica", "pendente + memo"), ("Worker", "varre a cada 100 ms"),
              ("Simulador", "recibo no ledger 2"), ("post ou void", "SETTLED ou REVERSED")]
    parts = ['<svg viewBox="0 0 680 190" xmlns="http://www.w3.org/2000/svg"><defs>'
             '<marker id="a" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8 Z" fill="#1e3a5f"/></marker>'
             '<marker id="m" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8 Z" fill="#78716c"/></marker></defs>'
             '<text x="0" y="14" font-size="12" font-weight="600">Autorização — percorrida pelo ensaio</text>'
             '<g font-size="11" font-weight="600" fill="#1c1917">']
    for i, (title, sub) in enumerate(top):
        x = i * 114
        parts.append(box(x, title, sub))
        if i < len(top) - 1:
            parts.append(arrow(x + 102, x + 112, 50))
    parts.append('<text x="0" y="112" font-size="12" font-weight="600" fill="#57534e">Destino externo — presente no processo, sem tráfego neste k6</text>')
    for i, (title, sub) in enumerate(bottom):
        x = i * 150
        parts.append(box(x, title, sub, y=122, w=130, h=48, strong=False))
        if i < len(bottom) - 1:
            parts.append(arrow(x + 132, x + 148, 146, "m", "#78716c"))
    parts.append("</g></svg>")
    return "".join(parts)


BODY = """
<header>
  <div class="rule"></div>
  <h1>Ensaio de 2.000 req/s em 10 minutos — TigerBeetle</h1>
  <p class="meta">Conta quente 1, modo misto, duas APIs e TigerBeetle 0.17.9 (uma réplica, cache de 2 GiB). Volume recriado, 1.000.000 de clientes semeados, k6 com chegada aberta dentro da rede do Compose. 1 de outubro de 2026, 12:26–12:36, horário de Brasília. Commit 6d72255.</p>
</header>

<div class="info">
  <strong>A meta passou nos quatro limiares.</strong>
  p99 de 74 ms, nenhuma falha HTTP, todos os checks APPROVED e nenhuma iteração descartada. O k6 encerrou com código 0. A mesma carga disparada do host macOS teve 4 conexões fechadas (EOF) em 1,2 milhão e não passou; ver Limites.
</div>

<div class="kpis">
  <div class="kpi ok"><b>74 ms</b><span>p99 HTTP ponta a ponta</span></div>
  <div class="kpi"><b>1.999,96/s</b><span>Taxa concluída</span></div>
  <div class="kpi ok"><b>1.200.001</b><span>Respostas APPROVED</span></div>
  <div class="kpi ok"><b>0</b><span>Falhas HTTP e descartes</span></div>
</div>

<section class="keep">
  <h2>Latência HTTP do ensaio</h2>
  <p class="muted">Mediana, p95, p99 e máximo de http_req_duration. A linha tracejada é a meta de p99 abaixo de 300 ms. Eixo vertical em milissegundos.</p>
  <figure>{bars}<figcaption>Fonte: reports/load-tb-10m-mixed-innet.json · k6 · 10m00s · api:8080 e api2:8080</figcaption></figure>
</section>

<section class="keep">
  <h2>Vazão sustentada</h2>
  <p class="muted">Requisições concluídas por segundo em cada minuto do relógio do k6. A linha tracejada é a taxa pedida. Eixo vertical: req/s, começando em zero.</p>
  <figure>{rate}<figcaption>Eixo horizontal: minuto do ensaio. Fonte: progresso do k6 por segundo.</figcaption></figure>
</section>

<section class="keep">
  <h2>Requisições em voo</h2>
  <p class="muted">Maior número de VUs ocupados numa amostra por segundo, em cada minuto. Com chegada aberta, VU ocupado é requisição esperando resposta. Eixo vertical: VUs (limite de 4.000).</p>
  <figure>{busy}<figcaption>O pico do minuto 8 é a maior travada amostrada; as requisições em voo escoaram sem descarte. Fonte: progresso do k6.</figcaption></figure>
</section>

<section class="keep">
  <h2>Critério do ensaio</h2>
  <table>
    <thead><tr><th>Critério</th><th class="num">Meta</th><th class="num">Medido</th><th>Resultado</th></tr></thead>
    <tbody>
      <tr><td>p99 HTTP</td><td class="num">&lt; 300 ms</td><td class="num">74,4 ms</td><td class="pass">Passou</td></tr>
      <tr><td>Falhas HTTP</td><td class="num">0</td><td class="num">0</td><td class="pass">Passou</td></tr>
      <tr><td>Checks APPROVED</td><td class="num">100%</td><td class="num">100%</td><td class="pass">Passou</td></tr>
      <tr><td>Iterações descartadas</td><td class="num">0</td><td class="num">0</td><td class="pass">Passou</td></tr>
    </tbody>
  </table>
</section>

<section class="keep">
  <h2>Todos os ensaios até passar</h2>
  <table>
    <thead><tr><th>Ensaio (reports/load-*.json)</th><th>Cenário</th><th class="num">p99 ms</th><th class="num">Falhas</th><th class="num">Descartes</th><th>Resultado</th></tr></thead>
    <tbody>{runs}</tbody>
  </table>
  <p>O que mudou entre a falha e a aprovação: o cache do TigerBeetle passou de 512 MiB para 2 GiB, e a API passou a juntar as criações de 5 ms num pedido só. Sem a janela, o cliente manda cerca de uma transferência por pedido, e o custo por pedido travava o ledger. Uma sonda direta ao TigerBeetle, sem HTTP, a 2.000 transferências/s, mediu p99 de 1,11 s com 512 MiB, 76 ms com 2 GiB e 34 a 51 ms com 2 GiB e a janela. Todo 503 desses ensaios é resultado desconhecido; nenhum virou recusa.</p>
</section>

<section class="keep">
  <h2>Fluxo do autorizador</h2>
  <p>Não há fila, lote de correção nem lock na aplicação: o TigerBeetle confere saldo (debits_must_not_exceed_credits) e id no mesmo commit, e cada transferência do pedido tem resultado próprio. A janela de 5 ms só existe por vazão.</p>
  <figure>{flow}<figcaption>Timeout de 1,5 s na API vira 503 (resultado desconhecido). O destino externo grava reserva e referência numa cadeia atômica; o TigerBeetle resolve uma reserva no máximo uma vez.</figcaption></figure>
</section>

<section class="keep">
  <h2>O que ficou gravado</h2>
  <table>
    <thead><tr><th>Auditoria com tráfego parado, depois da carga e das falhas</th><th class="num">Valor</th></tr></thead>
    <tbody>
      <tr><td>Contas de clientes</td><td class="num">1.000.000</td></tr>
      <tr><td>Saldos inválidos (negativos ou fora de int64)</td><td class="num">0</td></tr>
      <tr><td>Erro de conservação (dinheiro fora da emissão)</td><td class="num">0</td></tr>
      <tr><td>Linhas de histórico conferidas (contas 1 a 3)</td><td class="num">2.520.827</td></tr>
      <tr><td>Prefixos inválidos</td><td class="num">0</td></tr>
      <tr><td>Reservas externas sem liquidação</td><td class="num">0</td></tr>
    </tbody>
  </table>
  <p class="small">Fonte: reports/audit-tigerbeetle.json, 2026-10-01T15:38:29Z. A auditoria repete o histórico de cada saldo das contas escolhidas e compara o último com a conta.</p>
</section>

<section class="keep">
  <h2>Falhas injetadas</h2>
  <table>
    <thead><tr><th>Cenário</th><th>Resultado</th></tr></thead>
    <tbody>
      <tr><td>Resposta commitada perdida</td><td class="pass">Repetição devolve APPROVED, sem segundo efeito</td></tr>
      <tr><td>SIGKILL da api com 200 pedidos</td><td class="pass">Todos resolvidos na api2</td></tr>
      <tr><td>TigerBeetle pausado</td><td class="pass">503 em 1.506 ms, nunca recusa; mesmo id APPROVED depois</td></tr>
      <tr><td>TigerBeetle morto e reiniciado</td><td class="pass">Dados persistiram; mesmo id APPROVED</td></tr>
    </tbody>
  </table>
  <p class="small">Fonte: reports/faults.json, 203 operações únicas. No SIGKILL, os 200 pedidos já tinham terminado; o cenário não pegou pedido em andamento.</p>
</section>

<div class="note">
  <strong>Limites deste relatório.</strong>
  Uma máquina e uma réplica numa VM do Docker Desktop (8 CPUs, 7,6 GiB); o k6 dividiu a CPU com o sistema. O ensaio que passou começou com ledger limpo: num volume com ~5 milhões de transferências, logo após reiniciar, um teste de 60 s travou 10 s, e os 10 minutos não foram repetidos nesse volume. Do host, pelo encaminhamento de portas do Docker Desktop, houve 4 EOF em 1,2 milhão (p99 de 148 ms); um controle só HTTP pelo mesmo caminho teve zero, e a causa não foi isolada. Detalhes em reports/tigerbeetle-2026-10-01.md.
</div>
"""


def main():
    body = BODY.format(
        bars=svg_bars((5.3, 27.5, 74.4, 993.6), 1000),
        rate=svg_line(MINUTES, RATE, 2400, "#1e3a5f", ref=2000, ref_label="2.000 req/s"),
        busy=svg_line(MINUTES, BUSY_VUS, 1600, "#44403c"),
        runs=runs_table(),
        flow=flow(),
    )
    OUT.write_text(
        '<!DOCTYPE html><html lang="pt-BR"><head><meta charset="utf-8">'
        f"<title>Ensaio de 2.000 req/s em 10 minutos — TigerBeetle</title>{STYLE}</head><body>{body}</body></html>",
        encoding="utf-8",
    )
    print(OUT)


if __name__ == "__main__":
    main()
