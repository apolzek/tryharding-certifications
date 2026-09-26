# Postmortem: <título curto do incidente>

> **Blameless:** este documento descreve **o que o sistema permitiu que acontecesse**, não quem errou.
> Se uma pessoa conseguiu quebrar produção com uma mudança de uma linha, o problema é o processo que deixou essa linha passar.

| Campo | Valor |
|---|---|
| **Data** | AAAA-MM-DD |
| **Severidade** | SEV1 / SEV2 / SEV3 |
| **Duração do impacto** | hh:mm (do início do impacto até a mitigação) |
| **Tempo até detectar (TTD)** | hh:mm |
| **Tempo até mitigar (TTM)** | hh:mm |
| **Quem detectou** | alerta / cliente / pessoa olhando dashboard por acaso |
| **Incident commander** | nome |
| **Autores** | nomes |
| **Status** | rascunho / revisado / ações concluídas |

## Resumo

Duas ou três frases que um diretor entende: o que quebrou, quem foi afetado, por quanto tempo, e o que fizemos.

## Impacto

- Usuários/clientes afetados (número ou %).
- Requisições com erro, pedidos perdidos, SLO consumido (ex.: "consumiu 38% do error budget do mês").
- Impacto interno (horas de engenharia, pessoas acordadas à toa).

## Linha do tempo (UTC)

> Sempre em **UTC**, e diga isso no cabeçalho. Metade dos postmortems confusos é fuso horário misturado.

| Hora (UTC) | Evento |
|---|---|
| 12:03 | deploy X / mudança Y |
| 12:05 | **início do impacto** |
| 12:40 | cliente abre ticket |
| 12:52 | on-call começa a investigar |
| 13:10 | causa identificada |
| 13:15 | **mitigação** aplicada |
| 13:30 | confirmado que voltou ao normal |

## Causa raiz

O mecanismo técnico, com a evidência (query PromQL, trecho de config, log).
Use "5 porquês" até chegar em algo que **um processo** pode prevenir:

1. Por que ninguém foi avisado? → o alerta não disparou.
2. Por que não disparou? → ...
3. Por que ...? → ...

## Gatilho

O que disparou o problema (deploy, mudança de config, pico de tráfego, fim de certificado...).
Gatilho ≠ causa raiz: o gatilho é a faísca, a causa raiz é a gasolina.

## Detecção

- Como descobrimos? Um **alerta** deveria ter pego isso? Por que não pegou?
- Quanto tempo entre o início do impacto e alguém saber?

## Resolução

O que foi feito para mitigar e depois para corrigir de vez (com o diff da config).

## O que deu certo

- ...

## O que deu errado

- ...

## Onde tivemos sorte

- ...

## Ações

| # | Ação | Tipo | Dono | Prazo | Ticket |
|---|---|---|---|---|---|
| 1 | Corrigir ... | corrigir | | | |
| 2 | Adicionar teste `promtool test rules` para ... | prevenir | | | |
| 3 | Criar alerta de meta-monitoramento ... | detectar | | | |
| 4 | Atualizar runbook ... | mitigar | | | |

> Tipos: **prevenir** (não acontece de novo) · **detectar** (se acontecer, descobrimos em minutos) · **mitigar** (se acontecer, dói menos) · **corrigir** (o conserto em si).
> Uma boa lista tem pelo menos uma ação de **detectar**: "o monitoramento falhou calado" é quase sempre parte da história.

## Lições aprendidas

Uma ou duas frases que valem para outros times também.
