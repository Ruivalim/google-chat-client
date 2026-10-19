import { expect, test } from "@playwright/test";
import { calls, withBridge } from "./bridge";

const account = (page: import("@playwright/test").Page, email: string) =>
  page.getByRole("navigation", { name: "Contas Google" }).getByRole("button", { name: `Usar ${email}` });

test("prévia no navegador não oferece login", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("Abra o aplicativo desktop para conectar suas contas.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Importar credenciais Google" })).toBeDisabled();
});

test("sem projeto configurado, o primeiro passo é importar o JSON", async ({ page }) => {
  await withBridge(page, { configured: false, accounts: 0 });
  await page.goto("/");
  await expect(page.getByRole("button", { name: "Importar credenciais Google" })).toBeEnabled();
  await expect(page.getByRole("button", { name: "Conectar conta Google" })).toHaveCount(0);
});

test("cliques rápidos na faixa de contas terminam na última conta clicada", async ({ page }) => {
  // A primeira troca demora; a segunda responde na hora. A resposta atrasada não pode vencer.
  await withBridge(page, { delays: { "SelectAccount:222": 600 } });
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Chat" })).toBeVisible();

  await account(page, "ana@consultoria-b.com.br").click();
  await account(page, "ana@empresa-a.com").click();

  await expect(account(page, "ana@empresa-a.com")).toHaveAttribute("aria-pressed", "true");
  await page.waitForTimeout(900);
  await expect(account(page, "ana@empresa-a.com")).toHaveAttribute("aria-pressed", "true");
  await expect(page.locator(".workspace-footer")).toContainText("ana@empresa-a.com");
  expect((await calls(page, "SelectAccount")).map((c) => c.args[0])).toEqual(["222", "111"]);
  expect(await page.evaluate(() => window.bridge.state.activeAccountId)).toBe("111");
});

test("falha ao trocar de conta volta ao estado real e mostra o erro", async ({ page }) => {
  await withBridge(page, { failOnce: { "SelectAccount:222": "o chaveiro está bloqueado" } });
  await page.goto("/");
  await account(page, "ana@consultoria-b.com.br").click();
  await expect(page.getByText("O chaveiro está bloqueado")).toBeVisible();
  await expect(account(page, "ana@empresa-a.com")).toHaveAttribute("aria-pressed", "true");
});

test("cada conta tem as próprias conversas", async ({ page }) => {
  await withBridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  await account(page, "ana@consultoria-b.com.br").click();
  await expect(page.getByText("Uma conversa de cada vez")).toBeVisible();
  const spaces = await calls(page, "Spaces");
  expect(spaces.map((c) => c.args[0])).toEqual(["111", "222"]);
});

test("Enter envia, Shift+Enter quebra linha e composição de texto não envia", async ({ page }) => {
  await withBridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  const box = page.getByRole("textbox", { name: "Escrever mensagem" });

  await box.fill("primeira linha");
  await box.press("Shift+Enter");
  await box.pressSequentially("segunda");
  // Enter durante composição (IME) confirma o caractere, não a mensagem.
  await box.evaluate((el) =>
    el.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", isComposing: true, bubbles: true })),
  );
  expect(await calls(page, "SendMessage")).toHaveLength(0);
  await expect(box).toHaveValue("primeira linha\nsegunda");

  await box.press("Enter");
  await expect(page.getByText("segunda", { exact: false }).first()).toBeVisible();
  const [sent] = await calls(page, "SendMessage");
  expect(sent.args.slice(0, 4)).toEqual(["111", "spaces/dm111", "primeira linha\nsegunda", ""]);
  await expect(box).toHaveValue("");
  await expect(box).toBeFocused();
});

test("mensagem só com espaços não é enviada", async ({ page }) => {
  await withBridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  const box = page.getByRole("textbox", { name: "Escrever mensagem" });
  await box.fill("   \n  ");
  await box.press("Enter");
  await expect(page.getByRole("button", { name: "Enviar mensagem" })).toBeDisabled();
  expect(await calls(page, "SendMessage")).toHaveLength(0);
});

test("reenvio depois de falha usa o mesmo requestId e preserva o texto", async ({ page }) => {
  await withBridge(page, { failOnce: { SendMessage: "não foi possível alcançar o Google Chat" } });
  await page.goto("/");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  const box = page.getByRole("textbox", { name: "Escrever mensagem" });
  await box.fill("mensagem importante");
  await box.press("Enter");

  await expect(page.getByText("O texto foi preservado")).toBeVisible();
  await expect(box).toHaveValue("mensagem importante");
  await box.press("Enter");
  await expect(box).toHaveValue("");

  const [first, retry] = await calls(page, "SendMessage");
  expect(retry.args[4]).toBe(first.args[4]);
  expect(String(first.args[4])).toMatch(/^[0-9a-f-]{36}$/);
});

test("editar o texto depois da falha gera outro requestId", async ({ page }) => {
  await withBridge(page, { failOnce: { SendMessage: "falhou" } });
  await page.goto("/");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  const box = page.getByRole("textbox", { name: "Escrever mensagem" });
  await box.fill("versão um");
  await box.press("Enter");
  await expect(page.getByText("O texto foi preservado")).toBeVisible();
  await box.fill("versão dois");
  await box.press("Enter");
  const [first, second] = await calls(page, "SendMessage");
  expect(second.args[2]).toBe("versão dois");
  expect(second.args[4]).not.toBe(first.args[4]);
});

test("rascunho fica na conversa onde foi escrito", async ({ page }) => {
  await withBridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  await page.getByRole("textbox", { name: "Escrever mensagem" }).fill("rascunho para o Bruno");
  await page.getByRole("button", { name: /Time de Plataforma/ }).click();
  await expect(page.getByRole("textbox", { name: "Escrever mensagem" })).toHaveValue("");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  await expect(page.getByRole("textbox", { name: "Escrever mensagem" })).toHaveValue("rascunho para o Bruno");
});

test("resposta na thread vai para a thread certa", async ({ page }) => {
  await withBridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: /Time de Plataforma/ }).click();
  await page.getByRole("button", { name: "Abrir thread" }).first().click();
  const thread = page.getByRole("complementary", { name: "Thread" });
  await expect(thread.getByText("Resposta na thread")).toBeVisible();
  await thread.getByRole("textbox", { name: "Responder à thread" }).fill("concordo");
  await thread.getByRole("button", { name: "Enviar resposta à thread" }).click();
  await expect(thread.getByText("concordo")).toBeVisible();
  const [sent] = await calls(page, "SendMessage");
  expect(sent.args.slice(1, 4)).toEqual(["spaces/team111", "concordo", "spaces/team111/threads/t1"]);
  await thread.getByRole("button", { name: "Fechar thread" }).click();
  await expect(thread).toHaveCount(0);
});

test("link na mensagem abre fora do app e sem a pontuação final", async ({ page }) => {
  await withBridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: /Bruno Lima/ }).click();
  await page.getByRole("link", { name: "https://exemplo.com/tickets/42" }).click();
  await expect
    .poll(() => calls(page, "OpenLink"))
    .toEqual([{ method: "OpenLink", args: ["https://exemplo.com/tickets/42"] }]);
  expect(page.url()).toMatch(/127\.0\.0\.1:4173\/$/);
});

test("erro ao listar conversas oferece nova tentativa", async ({ page }) => {
  await withBridge(page, { failOnce: { Spaces: "não foi possível alcançar o Google Chat; confira a conexão" } });
  await page.goto("/");
  await expect(page.getByText("Não foi possível alcançar o Google Chat")).toBeVisible();
  await page.getByRole("button", { name: "Tentar novamente" }).click();
  await expect(page.getByRole("button", { name: /Bruno Lima/ })).toBeVisible();
  await expect(page.getByText("Não foi possível alcançar o Google Chat")).toHaveCount(0);
});

test("filtro e abas restringem a lista carregada", async ({ page }) => {
  await withBridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: "Espaços" }).click();
  await expect(page.getByRole("button", { name: /Bruno Lima/ })).toHaveCount(0);
  await page.getByRole("button", { name: "Tudo" }).click();
  await page.getByRole("textbox", { name: "Filtrar conversas carregadas" }).fill("zzz");
  await expect(page.getByText("Nenhuma conversa corresponde ao filtro.")).toBeVisible();
});

test("tooltip fica aberto enquanto o mouse está no ícone", async ({ page }) => {
  // O FluentProvider copia a className para o contêiner dos portais. Se ela for
  // a do layout, o contêiner cobre a tela, o ícone perde o hover e o tooltip pisca.
  await page.goto("/");
  const button = page.getByRole("button", { name: "Usar tema escuro" });
  await button.hover();
  await expect(page.getByRole("tooltip")).toBeVisible();
  for (let i = 0; i < 8; i++) {
    await page.waitForTimeout(100);
    await expect(page.getByRole("tooltip")).toHaveCount(1);
  }
  const box = await button.boundingBox();
  if (!box) throw new Error("botão sem posição");
  const onTop = await page.evaluate(
    ([x, y]) => document.elementFromPoint(x, y)?.closest("button")?.getAttribute("aria-label"),
    [box.x + box.width / 2, box.y + box.height / 2],
  );
  expect(onTop).toBe("Usar tema escuro");
});
