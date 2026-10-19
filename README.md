# Google Chat Client

**Retomada da implementação:** leia [HANDOFF.md](HANDOFF.md). Ele registra a escolha atual de MyGo + Fluent UI, os arquivos implementados, as verificações e as pendências. O texto abaixo preserva o contexto da pesquisa inicial.

Cliente para usar as conversas do Google Chat com uma interface de colaboração inspirada em Teams e Rocket.Chat. Chat é a prioridade; Calendar, Meet e Gmail entram depois.

O projeto está na etapa de pesquisa e escolha da base. A viabilidade foi conferida na documentação e no catálogo público de métodos da API; o acesso à conta corporativa ainda precisa ser validado.

## Direção recomendada

Criar um cliente em React e TypeScript usando os componentes Fuselage do Rocket.Chat e a API oficial do Google Chat com autorização do usuário. Essa é uma recomendação inicial, sujeita à validação do acesso corporativo e dos componentes publicados.

Para desktop, a proposta inicial é Electron com login no navegador do sistema. O sistema operacional de estreia ainda está em aberto.

## Pesquisa e primeira entrega

A [pesquisa de bases e viabilidade](docs/pesquisa-inicial.md) compara Fuselage, Fluent UI, Chatscope, Mattermost, Zulip, Element e clientes existentes de Google Chat. Também registra as limitações da API e o escopo proposto.

A primeira entrega deve permitir entrar com Google, listar conversas e ler mensagens reais. Isso valida a integração antes da implementação completa da interface.
