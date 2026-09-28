# Histórico de uso de IA

Registro resumido dos principais pedidos e decisões desta sessão; não é uma exportação literal completa. Não contém credenciais de serviços reais.

## Pedidos principais do candidato

1. Analisar o desafio de um autorizador Pix e escolher arquitetura, linguagem e ferramentas; suporte a hot accounts é obrigatório.
2. Avaliar a possibilidade de sustentar 2.000 requisições/s na mesma conta com p99 menor que 300 ms; explicar microlotes e coordenação distribuída.
3. Comparar monólito e microserviços.
4. Refazer o design em até quatro páginas cobrindo componentes, modelo de dados, concorrência, falhas, trade-offs e validações.
5. Implementar o projeto com testes unitários, Gherkin, cobertura, mutação, complexidade ciclomática, limites de módulos, dependências, métricas, procedimentos de QA e testes finais.
6. Continuar e concluir todo o trabalho.

## Uso da assistência

- Codex ajudou na elaboração do design e na escolha de Go/PostgreSQL com monólito modular.
- O design inicial foi salvo e commitado antes do código. Microlotes permaneceram condicionados à medição do baseline.
- A assistência escreveu código, testes, infraestrutura, scripts de carga/auditoria e documentação; executou os comandos no ambiente local.
- A skill local `skills/pix-quality/SKILL.md`, descoberta por AGENTS.md, registra os gates pedidos pelo candidato. A skill Ponytail orientou evitar componentes desnecessários; skill-creator orientou a skill local; PDF orientou a renderização e inspeção do design.
- A revisão identificou e corrigiu colisão potencial entre identidade de estorno e identidade de cliente, reservando `refund:`.
- Os resultados são extraídos de execuções reais. Falhas de compilação, infraestrutura ou metas são registradas e corrigidas ou declaradas; nenhum benchmark é inferido apenas da arquitetura.

## Responsabilidade na apresentação

O candidato deve revisar o código, entender READ COMMITTED e bloqueios ordenados, distinguir decisão financeira de erro técnico, explicar a fronteira de liquidação externa e reproduzir os comandos do README. A assistência não enviou e-mails aos avaliadores.
