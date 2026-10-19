# Direção de interface

Brief: aplicativo desktop de comunicação para trabalho, bonito e funcional, com referência de organização ao Teams. O usuário delegou a escolha da tecnologia e da base visual e aprovou aplicar antislop durante a construção.

Leitura: cliente de chat para uso diário no trabalho, com linguagem visual do Fluent UI. ENERGY 1 / RHYTHM 1 / MOTION 1.

- Fluent UI fornece controles e comportamento de teclado conhecidos de aplicativos de trabalho, reduzindo o custo de implementação.
- A faixa de contas fica sempre visível porque trocar entre empresas é uma ação essencial, com o email da conta atual presente no cabeçalho e no editor.
- A lista de conversas e o histórico ocupam painéis separados porque o usuário alterna entre localizar uma conversa e responder nela.
- O verde escuro identifica seleção e envio; textos e superfícies usam neutros para facilitar a leitura de históricos longos.
- Fontes do sistema preservam legibilidade e aparência familiar nas três plataformas, sem downloads de fontes.
- Mensagens usam linhas com autor e horário, evitando transformar cada mensagem em um cartão.
- Avatares são iniciais de dados reais das contas; não há fotos ou pessoas fictícias.
- A interface segue inicialmente o tema do sistema e oferece alternância entre claro e escuro.
- Movimento fica restrito ao feedback de controles e indicadores de trabalho em andamento.
- Controles de Calendar, Meet e Gmail serão acrescentados quando houver suas respectivas funções.

O executável usa uma janela desktop com webview do sistema. Os controles da interface são React e Fluent UI; não são widgets nativos escritos em Go.
