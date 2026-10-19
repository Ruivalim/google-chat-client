# O que a API do Google Chat permite

Levantado na documentação oficial e no [catálogo de descoberta v1](https://chat.googleapis.com/$discovery/rest?version=v1) (revisão 20261013). Serve para decidir o que o cliente pode prometer.

O app chama a API como o usuário que autorizou, com OAuth de aplicativo para computador. Não é um bot: não precisa de `chat.bot` nem aparece como app nas conversas. Fonte: [autenticação como usuário](https://developers.google.com/workspace/chat/authenticate-authorize-chat-user).

## Recursos

| Recurso | Método | O que saber |
| --- | --- | --- |
| Espaços, DMs e grupos | `spaces.list` | DM e grupo só existem depois da primeira mensagem. Paginado. |
| Nome de DM e grupo | `spaces.members.list` | DM e grupo vêm sem `displayName`; o nome sai dos membros. |
| Histórico | `spaces.messages.list` | Não traz avisos de sistema, como entrada de membro. Filtro por `thread.name`. |
| Enviar e responder em thread | `spaces.messages.create` | `requestId` torna o reenvio idempotente; `REPLY_MESSAGE_OR_FAIL` impede que a resposta vire conversa nova. |
| Editar e excluir | `spaces.messages.patch` / `delete` | Só as próprias mensagens, conforme permissão. |
| Reações | `spaces.messages.reactions` | Disponível com autenticação de usuário. |
| Arquivos | `media.upload` / `media.download` | Anexo do Drive vem como link e tem tratamento próprio. |
| Posição de leitura | `users.spaces.getSpaceReadState` / `updateSpaceReadState` | Só a do próprio usuário. Não é recibo de leitura dos colegas. Escopo `chat.users.readstate`. |
| Busca | `spaces.messages.search` | Exclui alguns tipos de mensagem e espaços silenciados. |
| Mudanças em tempo real | Workspace Events API + Cloud Pub/Sub | Assinatura expira (até 4 horas com o recurso no evento, até 7 dias só com o nome) e precisa ser renovada. |

Fontes: [métodos e escopos](https://developers.google.com/workspace/chat/authenticate-authorize), [spaces.list](https://developers.google.com/workspace/chat/api/reference/rest/v1/spaces/list), [messages.list](https://developers.google.com/workspace/chat/api/reference/rest/v1/spaces.messages/list), [busca](https://developers.google.com/workspace/chat/api/reference/rest/v1/spaces.messages/search), [estado de leitura](https://developers.google.com/workspace/chat/api/reference/rest/v1/users.spaces/updateSpaceReadState), [eventos de Chat](https://developers.google.com/workspace/events/guides/events-chat), [entrega de eventos](https://developers.google.com/workspace/events).

## Nome e email de quem escreve

Com autenticação de usuário, `sender.displayName` e `sender.email` vêm preenchidos quando a pessoa é membro do espaço ou já tem afinidade com quem chama (uma DM, por exemplo). Fora disso a API devolve só `name` (`users/{id}`) e `type`, e `isAnonymous` marca perfil apagado ou invisível. O catálogo diz que o `{id}` é o mesmo da People API e da Directory API. O app supõe que também é o `sub` do OpenID Connect (o ID da conta Google) e compara com ele para reconhecer as próprias mensagens e tirar a própria pessoa do nome das DMs. Isso não está escrito na documentação e precisa ser conferido no primeiro login real.

## O que não existe

- Indicador de digitação: não há método no catálogo v1.
- Presença dos colegas: a API de disponibilidade devolve só o estado do próprio usuário.
- Paridade completa com o cliente Google: avisos de sistema e cards de apps não vêm inteiros no histórico.
- Notificação com o app fechado: depende de algo recebendo eventos, o que hoje não existe.

Fonte: [disponibilidade do usuário](https://developers.google.com/workspace/chat/api/reference/rest/v1/users.availability/get).

## Atualização das conversas

Hoje o app consulta só a conversa aberta a cada 15 segundos, recuando até 2 minutos depois de erro. Gasta cota e tem atraso, mas não exige infraestrutura.

O próximo passo é a Workspace Events API com Pub/Sub: renovar assinaturas, descartar eventos repetidos e recuperar o que se perdeu durante a desconexão. Falta decidir entre o desktop consumir o Pub/Sub direto ou um serviço pequeno fazer isso por ele.

## Calendar, Meet e Gmail

Cada um entra com a própria autorização. Em app instalado, autorização incremental tem limitação documentada, então convém planejar os escopos de uma vez quando chegar a hora. Meet começa abrindo o link da reunião no navegador; a API REST do Meet administra espaços de reunião, não faz áudio e vídeo. Fonte: [API REST do Meet](https://developers.google.com/workspace/meet/api/guides/overview).
