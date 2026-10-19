# Como contribuir

Obrigado por querer ajudar. [English](CONTRIBUTING.md).

## Antes de começar

- Para um bug, abra uma issue com os passos para reproduzir, seu sistema e a versão do app.
- Para uma funcionalidade, abra uma issue antes, para combinarmos o escopo antes do código.
- Problema de segurança vai por relato privado, não por issue: veja [SECURITY.md](SECURITY.md).

## Preparando o ambiente

Siga a seção Desenvolver do [README](README.pt-BR.md#desenvolver): Go 1.27+, Bun e Node 24+, depois `make setup`. O `make preview` roda a interface no navegador sem conta Google, o que cobre a maior parte do trabalho de interface.

## Fazendo uma mudança

- Código novo vem com teste. Cubra o que quebra de verdade (entrada inválida, erro de rede, concorrência, limites), não só o caminho feliz. Os testes Go conversam com um Google simulado, e os de interface com uma ponte do MyGo simulada em `e2e/bridge.ts`; siga os que já existem.
- Todo texto que aparece para o usuário passa pelos catálogos em `locales/`: acrescente a chave no `en.json` (a fonte) e no `pt-BR.json`. Um teste falha quando falta chave em um deles.
- Rode `make check` antes de abrir o pull request. Ele roda o mesmo lint, testes e build do CI.
- Se um teste falhar, parta do princípio de que o código está errado. Se o teste é que estava errado, diga por quê no pull request.
- Pull requests pequenos e focados, explicando o motivo da mudança, não só o que mudou.

## Estilo

- Go: `gofmt` e `go vet`. As mensagens de erro ficam nos catálogos, começando em minúscula, como os erros do Go; a interface põe a maiúscula.
- TypeScript e CSS: Biome (`npm run fmt`). Componentes do Fluent UI, e as cores do tema em vez de cores novas.
- Comentários explicam o porquê, não o quê.

Ao contribuir, você concorda que a contribuição fica sob a [licença MIT](LICENSE).
