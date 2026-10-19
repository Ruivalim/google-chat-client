# Pesquisa inicial: cliente para Google Chat

Pesquisa realizada em 19 de outubro de 2026, usando documentação oficial, repositórios dos projetos e o documento público de descoberta da API do Google Chat. A comparação abaixo é uma avaliação técnica de adequação ao projeto. Nenhuma das bases foi instalada ou executada nesta etapa.

## Objetivo

Usar as mesmas conversas e a mesma conta corporativa do Google Chat em um aplicativo com navegação e organização próximas de Teams e Rocket.Chat.

Chat é a primeira entrega. Calendar, Meet e Gmail são expansões futuras.

## Recomendação

Criar um cliente em React e TypeScript com os componentes Fuselage do Rocket.Chat, conectado diretamente à API oficial do Google Chat com OAuth do usuário.

O Fuselage já oferece componentes como Message, Sidebar, Avatar e campos de entrada. A integração com Google, o estado das conversas, o envio de mensagens, a sincronização e as notificações continuam sendo trabalho do nosso cliente. Reutilizar esses componentes reduz o trabalho visual sem carregar o servidor Rocket.Chat.

O pacote declara licença MIT. O repositório antigo RocketChat/fuselage está arquivado e informa que os pacotes foram transferidos para o monorepo Rocket.Chat. A fonte atual consultada foi packages/fuselage desse monorepo. A versão encontrada no código foi 0.96.0; a escolha deve confirmar uma versão publicada compatível e fixar as dependências antes da implementação.

Fontes: [pacote atual do Fuselage](https://github.com/RocketChat/Rocket.Chat/blob/develop/packages/fuselage/package.json), [componentes disponíveis](https://github.com/RocketChat/Rocket.Chat/tree/develop/packages/fuselage/src/components), [aviso de transferência do repositório antigo](https://github.com/RocketChat/fuselage).

## Bases comparadas

| Projeto | O que podemos aproveitar | Limitação para este projeto | Avaliação |
| --- | --- | --- | --- |
| [Rocket.Chat / Fuselage](https://github.com/RocketChat/Rocket.Chat/tree/develop/packages/fuselage) | Componentes React específicos de chat, mensagens, avatares e navegação; pacote MIT. | É uma biblioteca de interface. Precisamos construir o comportamento e o adaptador Google. | Primeira escolha para reaproveitar a interface do Rocket.Chat. |
| [Microsoft Fluent UI e react-chat](https://github.com/microsoft/fluentui-contrib/tree/main/packages/react-chat) | Componentes de chat que o próprio projeto informa serem usados no Teams; bom ponto de partida para uma aparência Microsoft. | O pacote de chat consultado está em 0.2.2. Precisamos compor lista de conversas, editor, threads e integração; avaliar a API antes de adotá-lo. | Melhor alternativa se a aparência de Teams for decisiva. |
| [Chatscope Chat UI Kit](https://github.com/chatscope/chat-ui-kit-react) | Componentes de conversas, lista de mensagens e entrada de texto em React, com licença MIT. | Adaptar threads, organização de espaços e estados específicos do Google Chat. | Boa opção para uma primeira versão mais simples. |
| [Mattermost](https://github.com/mattermost/mattermost) | Aplicação com interface de colaboração completa em React. O arquivo de licença inclui webapp/ sob Apache 2.0. | O cliente, o estado e os eventos foram feitos para a API Mattermost. Adaptar implica uma mudança extensa. | Considerar se for indispensável partir de uma aplicação completa. |
| [Zulip](https://github.com/zulip/zulip) | Aplicação madura, com organização por tópicos e licença Apache 2.0. | O modelo de canais e tópicos exige adaptação para espaços, DMs e threads do Google. | Referência de organização; menor adequação como base direta. |
| [Element Web](https://github.com/element-hq/element-web) | Cliente web e desktop completo. | Usa Matrix JS SDK e depende do modelo Matrix. O repositório contém licenças AGPL/GPL e comercial; a seleção de código exige conferir a licença aplicável. | A adaptação ao Google tem custo alto. |
| [Google Chat Tauri](https://github.com/ankurk91/google-chat-tauri) | Aplicativo existente para Google Chat, com janela, bandeja e notificações; GPL-3.0-only. | Usa o serviço web existente. Sua abordagem não entrega por si uma interface de chat própria. | Referência para integração desktop; alternativa se a prioridade passar a ser empacotar o site. |

As avaliações de custo são inferências a partir da estrutura e das dependências documentadas, sem estimativa de prazo. Licenças devem ser verificadas nos arquivos e versões efetivamente reutilizados.

Fontes adicionais: [React Chat do Fluent UI](https://github.com/microsoft/fluentui-contrib/blob/main/packages/react-chat/README.md), [manifesto do pacote Fluent UI](https://github.com/microsoft/fluentui-contrib/blob/main/packages/react-chat/package.json), [licença do Mattermost](https://github.com/mattermost/mattermost/blob/master/LICENSE.txt), [estrutura do cliente Mattermost](https://docs.mattermost.com/developers/contribute/more-info/webapp), [estrutura do Rocket.Chat](https://github.com/RocketChat/Rocket.Chat/blob/develop/docs/backend-folder-structure.md).

### Por que evitar um fork completo do Rocket.Chat

O frontend faz parte da aplicação Meteor e inclui integrações com os serviços, modelos e APIs do próprio Rocket.Chat. Substituir a origem das mensagens exige adaptar autenticação, assinaturas de eventos, armazenamento, permissões e modelos de dados. Minha avaliação é que extrair componentes oferece uma manutenção mais simples para um cliente dedicado ao Google.

O projeto Rocket.Chat.Electron também merece uma distinção: é o aplicativo desktop que hospeda o cliente web Rocket.Chat. Reaproveitá-lo não traz automaticamente um frontend independente conectado ao Google.

Fonte: [Rocket.Chat.Electron](https://github.com/RocketChat/Rocket.Chat.Electron).

## O que a API oficial permite

A autenticação deve representar o usuário. A documentação permite operações em nome da conta que concedeu acesso. O login Google identifica a conta; os escopos OAuth de Chat autorizam o acesso às conversas. Um bot com chat.bot tem outro modelo de acesso.

Fonte: [autenticação como usuário](https://developers.google.com/workspace/chat/authenticate-authorize-chat-user).

| Recurso | Base oficial | Observação para implementação |
| --- | --- | --- |
| Listar espaços, DMs e grupos existentes | spaces.list | DMs e grupos só aparecem depois da primeira mensagem; paginar os resultados. |
| Carregar histórico | spaces.messages.list | A API não inclui mensagens de sistema como avisos de entrada de membros. |
| Enviar e responder em threads | spaces.messages.create e campo thread | Preservar os identificadores reais de mensagem, espaço e thread. |
| Editar e excluir mensagens | spaces.messages.patch / delete | Respeitar as permissões do usuário e as restrições de cada operação. |
| Reações | spaces.messages.reactions | Disponível com autenticação de usuário e escopos correspondentes. |
| Arquivos | media.upload / download | Implementar anexos do Chat; arquivos compartilhados por links do Drive podem exigir tratamento próprio. |
| Estado de leitura do próprio usuário | users.spaces.getSpaceReadState / updateSpaceReadState | Permite sincronizar a posição de leitura. Não equivale a recibos de leitura de todos os colegas. |
| Busca de mensagens | spaces.messages.search | Existe busca oficial. Exclui alguns tipos de mensagem e espaços silenciados; não substituir o carregamento de histórico por busca. |
| Receber alterações | Workspace Events API e Cloud Pub/Sub | Há assinaturas para espaços e para todos os espaços do usuário. Exigem renovação e recuperação de eventos perdidos. |

Fontes: [referência de métodos e escopos](https://developers.google.com/workspace/chat/authenticate-authorize), [listagem de espaços](https://developers.google.com/workspace/chat/api/reference/rest/v1/spaces/list), [listagem de mensagens](https://developers.google.com/workspace/chat/api/reference/rest/v1/spaces.messages/list), [busca](https://developers.google.com/workspace/chat/api/reference/rest/v1/spaces.messages/search), [estado de leitura](https://developers.google.com/workspace/chat/api/reference/rest/v1/users.spaces/updateSpaceReadState), [eventos de Chat](https://developers.google.com/workspace/events/guides/events-chat).

### Limites que afetam a experiência

- Não encontrei método público para enviar ou receber indicador de digitação no catálogo v1 consultado. Tratar como indisponível até existir documentação oficial correspondente.
- A API de disponibilidade consultada retorna o estado do próprio usuário autenticado. Ela não permite montar uma lista geral de colegas online.
- A listagem de histórico não retorna todos os avisos de sistema. A busca também tem exclusões documentadas. A interface não deve prometer equivalência completa com o cliente Google.
- As notificações de desktop dependem de o aplicativo ou algum serviço de suporte estar recebendo atualizações. Aplicativo encerrado exige uma decisão específica de execução em segundo plano.

Fontes: [catálogo público de descoberta v1](https://chat.googleapis.com/$discovery/rest?version=v1), [disponibilidade do usuário](https://developers.google.com/workspace/chat/api/reference/rest/v1/users.availability/get).

## Login e acesso corporativo

Para começar, precisamos de um projeto Google Cloud, Chat API habilitada e configurada, tela de consentimento e um cliente OAuth adequado à plataforma.

Para desktop, usar navegador do sistema, Authorization Code com PKCE e retorno por endereço de loopback. Tokens devem ficar sob responsabilidade da camada nativa e ser guardados com proteção do sistema operacional. As chamadas à API devem sair dessa camada; a interface recebe apenas os dados necessários.

O administrador do Workspace pode restringir aplicativos e escopos OAuth. Criar um projeto pessoal não garante acesso à conta da empresa. Um aplicativo interno depende de propriedade e configuração compatíveis com a organização. Essa validação deve acontecer antes do investimento na interface completa.

Fontes: [configuração e autorização Chat](https://developers.google.com/workspace/chat/authenticate-authorize-chat-user), [OAuth para desktop](https://developers.google.com/identity/protocols/oauth2/native-app), [controle de acesso a aplicativos no Workspace](https://knowledge.workspace.google.com/admin/apps/control-which-third-party-and-internal-apps-access-google-workspace-data).

### Escopos propostos

Os nomes abaixo têm o prefixo https://www.googleapis.com/auth/.

| Etapa | Escopos | Motivo |
| --- | --- | --- |
| Primeira leitura | chat.spaces.readonly e chat.messages.readonly | Listar conversas e carregar mensagens. |
| Primeiro envio | acrescentar chat.messages.create | Enviar uma mensagem sem ainda pedir edição e exclusão. |
| Chat completo | chat.spaces.readonly, chat.messages e chat.users.readstate | Ler, enviar, editar, excluir, reagir e atualizar a posição de leitura, conforme permissões. |
| Nomes e avatares adicionais | avaliar People API ou diretório conforme os dados retornados | Não presumir que todos os dados de perfil virão em cada resposta de Chat. |

Os escopos finais devem acompanhar as funções habilitadas. Calendário e email entram numa autorização futura própria; em aplicativos instalados, considerar a limitação documentada de autorização incremental ao planejar essa expansão.

## Atualização das conversas

Na primeira integração, usar consultas periódicas com intervalo limitado, paginação e recuo após erros. Isso permite validar o acesso sem montar infraestrutura de eventos. É uma solução inicial com atraso de atualização e consumo de cota, não uma promessa de atualização instantânea.

Para o aplicativo de uso diário, avaliar Workspace Events API com Cloud Pub/Sub. As assinaturas expiram: a documentação informa até quatro horas com dados do recurso, ou até sete dias quando o evento traz somente o nome do recurso. É necessário renovar assinaturas, eliminar eventos repetidos e recuperar alterações após desconexão.

Um serviço que recebe eventos e os entrega ao desktop é uma opção futura. A escolha entre um serviço e consumo local de Pub/Sub depende das permissões do projeto e da distribuição pretendida. Não há infraestrutura provisionada nesta etapa.

Fonte: [entrega e duração de eventos Workspace](https://developers.google.com/workspace/events).

## Primeiras entregas propostas

1. Validar acesso: configurar OAuth, entrar com a conta corporativa, listar conversas e ler mensagens reais. Tratar consentimento negado, bloqueio administrativo, token expirado e conversa vazia.
2. Validar uso de chat: envio por ação do usuário, histórico paginado, respostas em threads, reconexão e indicação clara de envio pendente ou falho.
3. Compor a interface com Fuselage: navegação de conversas, painel de mensagens e threads, teclado, tema, estados de carregamento e erros. Validar os componentes publicados antes de ampliar a interface.
4. Preparar uso diário: eventos e notificações, leitura sincronizada, reações, anexos e busca com suas limitações documentadas.

Proposta inicial para desktop: Electron com React e TypeScript. Tauri continua como alternativa caso tamanho do aplicativo seja prioritário. O primeiro sistema operacional ainda precisa ser definido; a escolha não muda a necessidade de validar OAuth e acesso ao Chat.

## Expansões posteriores

- Calendar: agenda, próximos compromissos e criação de eventos com links de reunião.
- Meet: começar com links para entrar em reuniões no navegador. A API REST administra espaços de reunião e dados de conferências; uma experiência própria de áudio e vídeo requer uma investigação adicional.
- Gmail: caixa de entrada e composição de mensagens em etapa separada, com os escopos e requisitos correspondentes.

Fonte para a distinção do Meet: [visão geral da API REST](https://developers.google.com/workspace/meet/api/guides/overview).

## Decisões ainda abertas

- Sistema operacional e formato da primeira distribuição.
- Possibilidade de criar/configurar o projeto Google Cloud e obter liberação de acesso à conta corporativa.
- Uso pessoal ou distribuição para outras pessoas, que afeta configuração OAuth e operação de eventos.
- Validação prática do Fuselage publicado e do acesso às conversas da empresa.

A pesquisa confirma capacidades documentadas. Uma autenticação real ainda é necessária para confirmar o comportamento na organização.
