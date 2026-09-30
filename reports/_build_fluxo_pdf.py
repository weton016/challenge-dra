#!/usr/bin/env python3
"""PDF: fluxo simples do cliente até o banco."""

from pathlib import Path

OUT = Path(__file__).with_name("fluxo-cliente-banco.html")


def pipeline():
    return """
<svg viewBox="0 0 680 118" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <marker id="p" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
      <path d="M0,0 L8,4 L0,8 Z" fill="#1e3a5f"/>
    </marker>
  </defs>
  <g font-size="11" font-weight="600" fill="#1c1917">
    <rect x="0" y="28" width="100" height="56" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
    <text x="50" y="52" text-anchor="middle">1. Cliente</text>
    <text x="50" y="68" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">POST JSON</text>
    <line x1="102" y1="56" x2="118" y2="56" stroke="#1e3a5f" marker-end="url(#p)"/>
    <rect x="120" y="28" width="100" height="56" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
    <text x="170" y="52" text-anchor="middle">2. API</text>
    <text x="170" y="68" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">chave + validar</text>
    <line x1="222" y1="56" x2="238" y2="56" stroke="#1e3a5f" marker-end="url(#p)"/>
    <rect x="240" y="28" width="100" height="56" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
    <text x="290" y="52" text-anchor="middle">3. Fila</text>
    <text x="290" y="68" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">até 4.096</text>
    <line x1="342" y1="56" x2="358" y2="56" stroke="#1e3a5f" marker-end="url(#p)"/>
    <rect x="360" y="28" width="100" height="56" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
    <text x="410" y="52" text-anchor="middle">4. Lote</text>
    <text x="410" y="68" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">até 64 pedidos</text>
    <line x1="462" y1="56" x2="478" y2="56" stroke="#1e3a5f" marker-end="url(#p)"/>
    <rect x="480" y="28" width="100" height="56" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
    <text x="530" y="52" text-anchor="middle">5. Banco</text>
    <text x="530" y="68" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">COMMIT</text>
    <line x1="582" y1="56" x2="598" y2="56" stroke="#1e3a5f" marker-end="url(#p)"/>
    <rect x="600" y="28" width="80" height="56" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
    <text x="640" y="52" text-anchor="middle">6. HTTP</text>
    <text x="640" y="68" text-anchor="middle" font-size="9" font-weight="400" fill="#57534e">200</text>
  </g>
  <text x="0" y="14" font-size="11" fill="#57534e">A resposta 200 só sai depois do passo 5. Nada é “aprovado” só na memória.</text>
</svg>
"""


def sequence():
    # Compact UML sequence: 5 actors
    xs = [50, 180, 310, 440, 580]
    names = ["Cliente", "API HTTP", "Fila", "Drenador", "PostgreSQL"]
    top, bottom = 36, 430
    lines = [
        (1, "POST /transfers + Bearer", 0, 1),
        (2, "401 se a chave falhar", 1, 0, True),
        (3, "valida JSON e IDs", 1, 1),
        (4, "coloca o pedido na fila", 1, 2),
        (5, "espera no canal (até 1,5 s)", 1, 1),
        (6, "junta até 64 (ou 15 ms)", 3, 2),
        (7, "BEGIN + lock 937513", 3, 4),
        (8, "lê saldo e IDs já gravados", 4, 3, True),
        (9, "decide cada pedido em memória", 3, 3),
        (10, "INSERT operations + entries", 3, 4),
        (11, "COMMIT (fsync)", 3, 4),
        (12, "ok, gravado", 4, 3, True),
        (13, "entrega o resultado no canal", 3, 1),
        (14, "HTTP 200 + decision", 1, 0, True),
    ]
    # y positions
    y0 = 58
    gap = 26
    parts = ['<svg viewBox="0 0 680 455" xmlns="http://www.w3.org/2000/svg">']
    parts.append('<text x="0" y="14" font-size="12" font-weight="600" fill="#1c1917">Diagrama de sequência — transferência local</text>')
    for i, (x, name) in enumerate(zip(xs, names)):
        parts.append(f'<rect x="{x-48}" y="22" width="96" height="22" rx="3" fill="#f5f5f4" stroke="#1e3a5f"/>')
        parts.append(f'<text x="{x}" y="37" text-anchor="middle" font-size="10" font-weight="600" fill="#1c1917">{name}</text>')
        parts.append(f'<line x1="{x}" y1="{top+10}" x2="{x}" y2="{bottom}" stroke="#d6d3d1"/>')
    for i, item in enumerate(lines):
        n, label, a, b, *rest = item
        dashed = rest[0] if rest else False
        y = y0 + i * gap
        x1, x2 = xs[a], xs[b]
        dash = ' stroke-dasharray="4 3"' if dashed else ""
        if a == b:
            parts.append(f'<rect x="{x1+8}" y="{y-10}" width="118" height="18" rx="2" fill="#fff" stroke="#d6d3d1"/>')
            parts.append(f'<text x="{x1+12}" y="{y+3}" font-size="8" fill="#1c1917">{n}. {label}</text>')
        else:
            left, right = min(x1, x2), max(x1, x2)
            arrow = "url(#s)" if x2 > x1 else "url(#sl)"
            parts.append(f'<line x1="{x1}" y1="{y}" x2="{x2}" y2="{y}" stroke="#1e3a5f"{dash} marker-end="{arrow}"/>')
            parts.append(f'<text x="{(left+right)/2}" y="{y-4}" text-anchor="middle" font-size="8" fill="#1c1917">{n}. {label}</text>')
    parts.append("""
  <defs>
    <marker id="s" markerWidth="7" markerHeight="7" refX="6" refY="3.5" orient="auto">
      <path d="M0,0 L7,3.5 L0,7 Z" fill="#1e3a5f"/>
    </marker>
    <marker id="sl" markerWidth="7" markerHeight="7" refX="6" refY="3.5" orient="auto">
      <path d="M0,0 L7,3.5 L0,7 Z" fill="#1e3a5f"/>
    </marker>
  </defs>
</svg>""")
    return "\n".join(parts)


def decide_flow():
    return """
<svg viewBox="0 0 680 210" xmlns="http://www.w3.org/2000/svg">
  <text x="0" y="14" font-size="12" font-weight="600" fill="#1c1917">Dentro do lote: como cada pedido vira APPROVED ou DECLINED</text>
  <rect x="0" y="28" width="130" height="40" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
  <text x="65" y="52" text-anchor="middle" font-size="10" font-weight="600">Este ID já existe?</text>
  <path d="M130 48 H 160" stroke="#1e3a5f" marker-end="url(#d)"/>
  <rect x="162" y="22" width="150" height="52" rx="4" fill="#fff" stroke="#d6d3d1"/>
  <text x="237" y="42" text-anchor="middle" font-size="10" font-weight="600">Sim</text>
  <text x="237" y="58" text-anchor="middle" font-size="9" fill="#57534e">mesmo JSON → devolve a decisão</text>
  <text x="237" y="70" text-anchor="middle" font-size="9" fill="#57534e">JSON diferente → 409</text>
  <text x="65" y="86" text-anchor="middle" font-size="9" fill="#57534e">Não</text>
  <path d="M65 68 V 100" stroke="#1e3a5f" marker-end="url(#d)"/>
  <rect x="0" y="102" width="130" height="40" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
  <text x="65" y="126" text-anchor="middle" font-size="10" font-weight="600">Contas existem?</text>
  <path d="M130 122 H 200" stroke="#1e3a5f" marker-end="url(#d)"/>
  <text x="155" y="116" font-size="9" fill="#57534e">Não</text>
  <rect x="202" y="102" width="140" height="40" rx="4" fill="#fff" stroke="#991b1b"/>
  <text x="272" y="120" text-anchor="middle" font-size="10" font-weight="600">DECLINED</text>
  <text x="272" y="134" text-anchor="middle" font-size="8" fill="#57534e">ACCOUNT_NOT_FOUND</text>
  <text x="78" y="156" font-size="9" fill="#57534e">Sim</text>
  <path d="M65 142 V 162" stroke="#1e3a5f" marker-end="url(#d)"/>
  <rect x="0" y="164" width="200" height="36" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
  <text x="100" y="186" text-anchor="middle" font-size="10" font-weight="600">Saldo da origem ≥ valor?</text>
  <path d="M200 182 H 248" stroke="#1e3a5f" marker-end="url(#d)"/>
  <text x="214" y="176" font-size="9" fill="#57534e">Sim</text>
  <rect x="252" y="164" width="120" height="36" rx="4" fill="#fff" stroke="#166534"/>
  <text x="312" y="186" text-anchor="middle" font-size="10" font-weight="600">APPROVED</text>
  <rect x="390" y="164" width="210" height="36" rx="4" fill="#fff" stroke="#991b1b"/>
  <text x="495" y="178" text-anchor="middle" font-size="9" font-weight="600">Não → DECLINED</text>
  <text x="495" y="192" text-anchor="middle" font-size="8" fill="#57534e">INSUFFICIENT_FUNDS · sem partidas</text>
  <defs>
    <marker id="d" markerWidth="7" markerHeight="7" refX="6" refY="3.5" orient="auto">
      <path d="M0,0 L7,3.5 L0,7 Z" fill="#1e3a5f"/>
    </marker>
  </defs>
</svg>
"""


def tables():
    return """
<svg viewBox="0 0 680 150" xmlns="http://www.w3.org/2000/svg">
  <text x="0" y="14" font-size="12" font-weight="600" fill="#1c1917">O que o COMMIT grava (aprovação local)</text>
  <rect x="0" y="28" width="210" height="110" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
  <text x="105" y="50" text-anchor="middle" font-size="11" font-weight="600">operations</text>
  <text x="12" y="70" font-size="9" fill="#57534e">1 linha por ID</text>
  <text x="12" y="86" font-size="9" fill="#57534e">decision APPROVED ou DECLINED</text>
  <text x="12" y="102" font-size="9" fill="#57534e">payload canônico (origem, destino, valor)</text>
  <text x="12" y="118" font-size="9" fill="#57534e">chave: transaction_id</text>
  <rect x="230" y="28" width="210" height="110" rx="4" fill="#f5f5f4" stroke="#1e3a5f"/>
  <text x="335" y="50" text-anchor="middle" font-size="11" font-weight="600">entries</text>
  <text x="242" y="70" font-size="9" fill="#57534e">2 linhas se APPROVED</text>
  <text x="242" y="86" font-size="9" fill="#57534e">0 linhas se DECLINED</text>
  <text x="242" y="102" font-size="9" fill="#57534e">−valor na origem, +valor no destino</text>
  <text x="242" y="118" font-size="9" fill="#57534e">saldo depois de cada partida</text>
  <rect x="460" y="28" width="210" height="110" rx="4" fill="#fff" stroke="#d6d3d1"/>
  <text x="565" y="50" text-anchor="middle" font-size="11" font-weight="600">outbox</text>
  <text x="472" y="70" font-size="9" fill="#57534e">só se o destino for externo</text>
  <text x="472" y="86" font-size="9" fill="#57534e">neste fluxo local: vazio</text>
  <text x="472" y="102" font-size="9" fill="#57534e">accounts não é reescrita</text>
  <text x="472" y="118" font-size="9" fill="#57534e">o saldo vigente é a última entry</text>
</svg>
"""


HTML = """<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<title>Do cliente até o banco</title>
<style>
  @page {{ size: A4; margin: 14mm 14mm 16mm; }}
  * {{ box-sizing: border-box; }}
  body {{
    margin: 0; font-family: "Helvetica Neue", Helvetica, Arial, sans-serif;
    color: #1c1917; font-size: 11px; line-height: 1.45;
    -webkit-print-color-adjust: exact; print-color-adjust: exact;
  }}
  h1 {{ font-size: 22px; margin: 0 0 6px; font-weight: 650; letter-spacing: -0.02em; }}
  h2 {{ font-size: 14px; margin: 16px 0 6px; font-weight: 650; break-after: avoid; }}
  p {{ margin: 0 0 8px; }}
  .rule {{ height: 2px; background: #1c1917; margin: 0 0 12px; }}
  .muted {{ color: #57534e; }}
  .small {{ font-size: 9.5px; color: #78716c; }}
  .keep {{ break-inside: avoid; page-break-inside: avoid; }}
  ol.steps {{ margin: 4px 0 10px; padding-left: 18px; }}
  ol.steps li {{ margin-bottom: 5px; }}
  table {{ width: 100%; border-collapse: collapse; margin: 4px 0 10px; page-break-inside: avoid; }}
  th {{ text-align: left; font-size: 10px; border-bottom: 1.5px solid #1c1917; padding: 5px 6px; }}
  td {{ border-bottom: 1px solid #e7e5e4; padding: 4px 6px; vertical-align: top; }}
  code, .mono {{ font-family: ui-monospace, Menlo, monospace; font-size: 10px; }}
  .box {{
    background: #f8fafc; border-left: 3px solid #1e3a5f;
    padding: 8px 10px; margin: 8px 0;
  }}
  .warn {{
    background: #fffbeb; border-left: 3px solid #92400e;
    padding: 8px 10px; margin: 8px 0;
  }}
  figure {{ margin: 6px 0 12px; }}
  figcaption {{ font-size: 9.5px; color: #78716c; margin-top: 2px; }}
  svg {{ width: 100%; height: auto; display: block; }}
  pre {{
    background: #f5f5f4; padding: 8px 10px; font-size: 10px;
    margin: 4px 0 10px; white-space: pre-wrap;
  }}
</style>
</head>
<body>
<header>
  <div class="rule"></div>
  <h1>Do cliente até o banco</h1>
  <p class="muted">Passo a passo de um POST /transfers local neste autorizador. Duas APIs (:8080 e :8081) fazem o mesmo caminho; as duas gravam no mesmo PostgreSQL.</p>
</header>

<section class="keep">
  <h2>Visão geral</h2>
  <p>O cliente manda um JSON com um ID que ele escolhe e guarda. A API não aprova na hora. Ela coloca o pedido numa fila, junta vários, grava tudo numa transação e <strong>só então</strong> responde. Se a resposta se perder, o cliente manda o mesmo ID de novo e recebe a decisão já salva, sem mover o dinheiro outra vez.</p>
  <figure>
    {pipeline}
    <figcaption>Seis estágios. O HTTP 200 é o último, depois do COMMIT.</figcaption>
  </figure>
</section>

<section class="keep">
  <h2>O que o cliente envia</h2>
  <p>URL: <span class="mono">POST http://127.0.0.1:8080/transfers</span> (ou :8081). Cabeçalho <span class="mono">Authorization: Bearer …</span> e corpo:</p>
  <pre>{{
  "transaction_id": "abc-1",
  "source_id": 2,
  "destination_id": 1,
  "amount_minor": 1
}}</pre>
  <p class="small">Valores em centavos. transaction_id: letras, números, _ e -, até 64 caracteres. Origem e destino são contas locais positivas e diferentes. Destino externo usa outro campo e não entra neste passeio.</p>
</section>

<section>
  <h2>Diagrama de sequência</h2>
  <p class="muted">Setas cheias vão para a frente. Setas tracejadas voltam. O “Drenador” é um goroutine só, por API: ele pega a fila e fala com o banco.</p>
  <figure>
    {sequence}
    <figcaption>UML de sequência do caminho feliz. Códigos de erro saem mais cedo (tabela no fim).</figcaption>
  </figure>
</section>

<section>
  <h2>Passo a passo</h2>
  <h2 style="font-size:12px;margin-top:8px">Na porta HTTP (ainda sem banco)</h2>
  <ol class="steps">
    <li><strong>Chega o POST.</strong> O servidor Go aceita a conexão (timeout de leitura/escrita 3 s).</li>
    <li><strong>Confere a chave.</strong> O Bearer tem de ser exatamente a API_KEY. Se não for, responde 401 e para. /healthz e /readyz passam sem chave.</li>
    <li><strong>Marca 1,5 s no relógio.</strong> Esse prazo vale para o pedido inteiro. Estourar vira erro técnico (503), nunca DECLINED.</li>
    <li><strong>Lê o JSON.</strong> No máximo 4 KB, sem campos extras. Valida ID, contas e valor positivo. Origem igual ao destino é inválido. Falha → 400 INVALID_REQUEST.</li>
    <li><strong>Enfileira.</strong> O pedido entra num canal em memória, com até 4.096 lugares. A goroutine do HTTP não grava SQL. Ela espera um recado num canalzinho só daquele pedido.</li>
    <li><strong>Fila cheia?</strong> Responde na hora 503 e pede para repetir o mesmo ID. Nada foi ao banco.</li>
  </ol>
  <h2 style="font-size:12px;margin-top:8px">O drenador junta um lote</h2>
  <ol class="steps" start="7">
    <li><strong>Pega o primeiro da fila</strong> e tenta encher até 64. Se a fila já está cheia, drena os 64 na hora. Se está vazia, espera no máximo 15 ms (BATCH_WINDOW_MS) por mais gente.</li>
    <li><strong>Abre uma transação SQL</strong> com prazo próprio de 1,2 s. Se o cliente desistir no meio, os outros do lote continuam: o COMMIT não é cancelado por um timeout HTTP.</li>
    <li><strong>Pede o lock 937513.</strong> Só um lote no mundo (as duas APIs) grava de cada vez. A outra espera.</li>
    <li><strong>Lê o saldo vigente</strong> da origem e do destino: não é a coluna accounts neste caminho, é a última linha de entries daquela conta (se não houver partida, usa o saldo inicial).</li>
    <li><strong>Lê operations</strong> com aqueles transaction_id. Se o ID já está lá, compara o JSON. Igual → devolve a decisão antiga. Diferente → conflito 409, sem mexer no resto do lote.</li>
  </ol>
</section>

<section class="keep">
  <figure>
    {decide}
    <figcaption>ledger.Decide: valor positivo, origem com saldo suficiente (igual pode), destino sem estourar o inteiro máximo.</figcaption>
  </figure>
</section>

<section>
  <h2 style="font-size:12px">Gravação no PostgreSQL</h2>
  <ol class="steps" start="12">
    <li><strong>Monta as partidas em memória.</strong> Aprovação: −valor na origem e +valor no destino, cada uma com a próxima sequência e o saldo depois da linha. Negativa: zero partidas, mas a operação DECLINED ainda é gravada (aquele ID fica terminal).</li>
    <li><strong>INSERT em operations.</strong> Se outro processo gravou o mesmo ID no meio, o ON CONFLICT ignora; o lote relê e tenta de novo (até 3 vezes).</li>
    <li><strong>INSERT em entries</strong> das partidas aprovadas.</li>
    <li><strong>INSERT na outbox</strong> só se settlement for PENDING (destino externo). Transferência local não entra aqui.</li>
    <li><strong>COMMIT.</strong> Aí o disco confirma (fsync ligado). Antes disso não existe APPROVED para o cliente.</li>
    <li><strong>Entrega no canal.</strong> Cada HTTP parado acorda, monta o JSON e responde 200.</li>
  </ol>
  <figure>
    {tables}
    <figcaption>Três tabelas. A linha de accounts não é atualizada neste caminho; o extrato e o saldo leem a última partida.</figcaption>
  </figure>
</section>

<section class="keep">
  <h2>Exemplo numérico</h2>
  <p>Conta 2 manda 1 centavo para a conta 1. Saldo da 2 era 1.000.000.000.000.</p>
  <table>
    <thead><tr><th>Tabela</th><th>O que fica</th></tr></thead>
    <tbody>
      <tr><td>operations</td><td>id abc-1, APPROVED, INTERNAL, payload com origem 2, destino 1, amount 1</td></tr>
      <tr><td>entries (origem)</td><td>conta 2, delta −1, sequence N+1, balance_after = saldo anterior − 1</td></tr>
      <tr><td>entries (destino)</td><td>conta 1, delta +1, sequence M+1, balance_after = saldo anterior + 1</td></tr>
      <tr><td>Resposta HTTP</td><td class="mono">{{"transaction_id":"abc-1","decision":"APPROVED","settlement_status":"INTERNAL"}}</td></tr>
    </tbody>
  </table>
</section>

<section class="keep">
  <h2>O que o cliente pode receber</h2>
  <table>
    <thead><tr><th>HTTP</th><th>Quando</th><th>O que fazer</th></tr></thead>
    <tbody>
      <tr><td>200 APPROVED</td><td>COMMIT feito, duas partidas</td><td>Pronto</td></tr>
      <tr><td>200 DECLINED</td><td>COMMIT feito, zero partidas (saldo, conta, overflow)</td><td>Aquele ID morreu; novo dinheiro pede novo ID</td></tr>
      <tr><td>400</td><td>JSON ou regra de formato</td><td>Corrigir o pedido</td></tr>
      <tr><td>401</td><td>Chave errada</td><td>Corrigir o Bearer</td></tr>
      <tr><td>409</td><td>Mesmo ID, outro JSON</td><td>Não reutilizar o ID</td></tr>
      <tr><td>503</td><td>Fila cheia, timeout, banco lento, COMMIT incerto</td><td>Repetir o <em>mesmo</em> ID. 503 nunca é negativa de negócio</td></tr>
    </tbody>
  </table>
</section>

<div class="warn">
  <strong>Fila não é banco.</strong>
  Se a API morrer depois de enfileirar e antes do COMMIT, o PostgreSQL desfaz a transação. O cliente não recebeu 200: manda o mesmo ID. Se morrer depois do COMMIT e antes da resposta, o dinheiro já moveu: o reenvio só relê operations.
</div>

<div class="box">
  <strong>Este PDF descreve só a transferência local.</strong>
  GET /accounts/{{id}}/balance lê a última partida. Destino externo debita a origem, credita uma conta técnica (−1 a −32), grava outbox e um worker entrega no simulador. Consulta e Pix externo não aparecem no diagrama de sequência acima.
</div>
</body>
</html>
"""


def main():
    # Escape remaining braces in HTML that aren't placeholders
    html = HTML.format(
        pipeline=pipeline(),
        sequence=sequence(),
        decide=decide_flow(),
        tables=tables(),
    )
    OUT.write_text(html, encoding="utf-8")
    print(OUT)


if __name__ == "__main__":
    main()
