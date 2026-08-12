import { strict as assert } from "node:assert";
import { test } from "node:test";
import { repoLabel } from "./repo-url.ts";

test("https adresinden kullanıcı/depo çıkarır", () => {
  assert.deepEqual(repoLabel("https://github.com/codeeer/agent-coder.git"), {
    host: "github.com",
    path: "codeeer/agent-coder",
  });
});

test("sondaki .git ve eğik çizgi atılır", () => {
  assert.equal(repoLabel("https://github.com/a/b/").path, "a/b");
  assert.equal(repoLabel("https://github.com/a/b.GIT").path, "a/b");
});

test("adresteki kimlik bilgisi ekrana çıkmaz", () => {
  // Sızması en kolay yer burası: token adresin içinde geliyor.
  const { host, path } = repoLabel("https://kullanici:gizli@github.com/a/b.git");
  assert.equal(host, "github.com");
  assert.equal(path, "a/b");
  assert.ok(!`${host}${path}`.includes("gizli"));
});

test("ssh biçimi tanınır", () => {
  assert.deepEqual(repoLabel("git@github.com:codeeer/agent-coder.git"), {
    host: "github.com",
    path: "codeeer/agent-coder",
  });
});

test("port taşıyan sunucu korunur", () => {
  assert.deepEqual(repoLabel("http://localhost:3000/a/b.git"), {
    host: "localhost:3000",
    path: "a/b",
  });
});

test("yol yoksa sunucu adı yola düşer", () => {
  assert.deepEqual(repoLabel("https://example.com"), {
    host: "example.com",
    path: "example.com",
  });
});

test("ayrıştırılamayan adres olduğu gibi kalır", () => {
  assert.deepEqual(repoLabel("bu bir adres değil"), {
    host: "",
    path: "bu bir adres değil",
  });
});

test("boş girdi boş döner", () => {
  assert.deepEqual(repoLabel("   "), { host: "", path: "" });
});
