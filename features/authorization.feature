# language: pt
Funcionalidade: Autorizar Pix com saldo consistente
  Contexto:
    Dado contas com saldo inicial de 100 centavos

  Cenário: Transferência interna e extrato
    Quando transfiro 100 centavos com a chave "pagamento"
    Então a decisão é "APPROVED"
    E o saldo da origem é 0 centavos
    E existem 2 partidas para "pagamento"

  Cenário: Saldo insuficiente
    Quando transfiro 101 centavos com a chave "sem-saldo"
    Então a decisão é "DECLINED"
    E o saldo da origem é 100 centavos
    E existem 0 partidas para "sem-saldo"

  Cenário: Reenvio após resposta perdida
    Quando transfiro 40 centavos com a chave "repetido"
    E transfiro 40 centavos com a chave "repetido"
    Então a decisão é "APPROVED"
    E o saldo da origem é 60 centavos
    E existem 2 partidas para "repetido"

  Cenário: Reutilização indevida de chave
    Quando transfiro 40 centavos com a chave "conflito"
    E transfiro 41 centavos com a chave "conflito"
    Então o HTTP é 409
    E o saldo da origem é 60 centavos

  Cenário: Cem débitos simultâneos em duas instâncias
    Quando envio 100 débitos simultâneos de 10 centavos
    Então exatamente 10 são aprovados
    E o saldo da origem é 0 centavos

  Cenário: Rejeição externa gera apenas um estorno
    Quando envio 40 centavos para destino externo rejeitado
    E processo a outbox e repito a rejeição
    Então o saldo da origem é 100 centavos
    E a liquidação é "REVERSED"
