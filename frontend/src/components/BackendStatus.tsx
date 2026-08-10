"use client";

import { useEffect, useState } from "react";
import { ApiError, api } from "@/lib/api";
import type { HealthResponse } from "@/lib/types";
import { Card, Field, StatusDot } from "@/components/ui/primitives";

type State =
  | { kind: "loading" }
  | { kind: "ok"; health: HealthResponse }
  | { kind: "error"; message: string };

/** Backend'e bağlantıyı doğrular ve sürüm bilgisini gösterir. */
export function BackendStatus() {
  const [state, setState] = useState<State>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;

    api
      .health()
      .then((health) => {
        if (!cancelled) setState({ kind: "ok", health });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        const message =
          err instanceof ApiError ? err.message : "Bilinmeyen hata";
        setState({ kind: "error", message });
      });

    // Bileşen sökülürse gecikmiş yanıt state'i güncellemesin.
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <Card>
      <div className="flex items-center gap-2">
        <StatusDot
          tone={
            state.kind === "ok"
              ? "success"
              : state.kind === "error"
                ? "danger"
                : "neutral"
          }
          pulse={state.kind === "loading"}
        />
        <span className="text-[13px] font-medium">
          {state.kind === "loading" && "Backend kontrol ediliyor…"}
          {state.kind === "ok" && "Backend bağlı"}
          {state.kind === "error" && "Backend'e ulaşılamıyor"}
        </span>
      </div>

      {state.kind === "ok" && (
        <dl className="mt-4 grid grid-cols-3 gap-4">
          <Field label="Durum" value={state.health.status} mono />
          <Field label="Sürüm" value={state.health.version} mono />
          <Field label="Ortam" value={state.health.env} mono />
        </dl>
      )}

      {state.kind === "error" && (
        <div className="mt-2">
          <p className="text-[13px] text-ink-2">{state.message}</p>
          <p className="mt-2 text-[12px] text-ink-3">
            Servisler ayakta mı? <code>make ps</code> ile kontrol edin.
          </p>
        </div>
      )}
    </Card>
  );
}
