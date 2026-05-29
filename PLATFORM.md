# Manifesto das Quatro Responsabilidades da Plataforma de IA da RBX

A RBX não deve tratar IA como uma coleção de agentes, prompts, modelos ou integrações isoladas.

A RBX deve tratar IA como uma plataforma operacional governada, composta por responsabilidades separadas, auditáveis e combináveis.

A tese central é simples:

> Avaliação, controle, execução e governança não devem viver no mesmo lugar.

Quando essas responsabilidades são misturadas, a plataforma se torna frágil, difícil de auditar e perigosa para sistemas críticos. Quando são separadas, cada camada pode evoluir com clareza, mantendo rastreabilidade, segurança e capacidade de decisão.

## 1. TruthMetal: responsabilidade pela verdade

TruthMetal é a camada de groundtruth da RBX.

Sua função é responder:

> O agente está correto?

TruthMetal deve cuidar de:

- datasets de avaliação;
- golden cases;
- benchmarks;
- eval suites;
- scorers;
- métricas de qualidade;
- evidências esperadas;
- regressão de comportamento;
- relatórios de confiabilidade.

TruthMetal não é o runtime dos agentes. TruthMetal não decide produção em tempo real. TruthMetal mede, compara e produz evidência.

## 2. Thalamus: responsabilidade pelo controle

Thalamus é a camada de controle semântico e operacional da IA na RBX.

Sua função é responder:

> Este agente pode operar agora, neste contexto, com este risco?

Thalamus deve cuidar de:

- políticas de uso;
- roteamento de modelos;
- guardrails;
- validações;
- observabilidade;
- tracing;
- limites de contexto;
- gates de produção;
- fallback;
- intervenção humana;
- integração com evals e telemetry.

Thalamus não é o dono da verdade. Ele consome evidências do TruthMetal e aplica decisões operacionais.

## 3. Agent Orchestration Plane: responsabilidade pela execução

O Agent Orchestration Plane é a camada de coordenação dos agentes da RBX.

Sua função é responder:

> O que deve ser executado, por qual agente, em qual ordem, com quais condições de parada?

Ele deve cuidar de:

- catálogo de agentes;
- missões;
- planos;
- delegação;
- ordem de execução;
- condições de execução;
- retries;
- loops limitados;
- término de missão;
- memória operacional;
- sessões isoladas;
- coordenação entre agentes humanos e artificiais.

O Agent Orchestration Plane não deve ser confundido com governança, evals ou roteamento de LLM. Sua responsabilidade é coordenar trabalho.

## 4. RBX Governance: responsabilidade pela decisão institucional

RBX Governance é a camada institucional de decisão.

Sua função é responder:

> Por que decidimos isso, quem decidiu, quando, com qual impacto e quais sistemas são afetados?

RBX Governance deve cuidar de:

- ADRs;
- policies;
- standards;
- decision registry;
- mission registry;
- ownership;
- relações entre sistemas;
- supersedes e depreciações;
- auditoria arquitetural;
- rastreabilidade estratégica.

RBX Governance não executa agentes e não avalia outputs diretamente. Ela registra e organiza as decisões que tornam a plataforma compreensível e auditável.

## Separação fundamental

A RBX deve preservar esta separação:

```
TruthMetal          → O agente está correto?
Thalamus            → O agente pode operar?
Agent Orch. Plane   → O agente deve executar?
RBX Governance      → Por que decidimos isso?
```

Esta divisão impede que a plataforma de IA vire um bloco opaco.

Ela permite que a RBX construa sistemas de IA com:

- avaliação objetiva;
- controle operacional;
- execução coordenada;
- governança auditável;
- segurança em ambientes críticos;
- evolução técnica sem perda de responsabilidade.

## Princípio final

A RBX não busca apenas usar IA.

A RBX busca construir uma infraestrutura de IA confiável, governada e operacionalmente segura.

Modelos mudam. Frameworks mudam. Agentes mudam.

Mas as quatro responsabilidades permanecem:

> verdade, controle, execução e decisão.

---

## Posição deste repositório

**`truthmetal`** É a camada de Verdade da plataforma RBX.

Este repositório detém a responsabilidade pela avaliação objetiva de agentes e modelos. Aqui vivem datasets de avaliação, golden cases, benchmarks, eval suites e métricas de confiabilidade. TruthMetal produz evidência — não opera agentes em produção, não toma decisões de roteamento em tempo real.

A evidência produzida pelo TruthMetal é consumida pelo Thalamus para calibrar gates de produção e pelo RBX Governance para rastrear confiabilidade ao longo do tempo. Nenhuma decisão de "este agente pode ir para produção" deve ser tomada sem evidência do TruthMetal.
