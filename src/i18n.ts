import { locale as appLocale, isMyGo } from "mygo-runtime";
import { type Messages, messages, messagesIn } from "./locales";

// "" segue o idioma do app (no desktop, o do sistema ou o que o Go guardou) ou o
// do navegador (na prévia). Um idioma escolhido nas configurações fica aqui até
// a página ouvir a troca.
let chosen = "";
try {
  if (!isMyGo()) chosen = localStorage.getItem("language") ?? "";
} catch {
  // Sem localStorage a prévia segue o navegador.
}

export function chosenLanguage() {
  return chosen;
}

export function setChosenLanguage(tag: string) {
  chosen = tag;
  try {
    if (!isMyGo()) localStorage.setItem("language", tag);
  } catch {
    // Só a prévia guarda aqui; no desktop quem guarda é o Go.
  }
}

/** As mensagens no idioma da tela. Chame a cada render, não guarde o retorno. */
export function t(): Messages {
  return chosen ? messagesIn(chosen) : messages;
}

/** O idioma da tela, para datas, números e maiúsculas. */
export function currentLocale(): string {
  if (chosen) return chosen;
  // A mesma ordem do catálogo: o idioma do app, senão o do navegador.
  return (isMyGo() && appLocale()) || navigator.language || "en";
}
