"use client";

import { useEffect, useRef, useState } from "react";
import { api } from "./api";
import type { RunEvent, RunStatus } from "./types";

/**
 * Bir çalıştırmanın canlı olay akışını dinler.
 *
 * Backend önce veritabanındaki geçmişi, sonra canlı olayları gönderir; bu
 * yüzden sayfa yenilense de o ana kadarki ilerleme kaybolmaz.
 *
 * Olaylar `seq` ile tekilleştirilir: yeniden bağlanmada geçmiş baştan gelir.
 */
export function useRunEvents(runId: string, enabled: boolean) {
  const [events, setEvents] = useState<RunEvent[]>([]);
  const [terminalStatus, setTerminalStatus] = useState<RunStatus | null>(null);
  const [connected, setConnected] = useState(false);

  // seq kümesi render'ları tetiklememeli; ref'te tutuluyor.
  const seen = useRef<Set<number>>(new Set());

  useEffect(() => {
    if (!enabled) return;

    const source = new EventSource(api.runs.eventsUrl(runId));

    source.onopen = () => setConnected(true);

    source.onmessage = (msg) => {
      let e: RunEvent;
      try {
        e = JSON.parse(msg.data) as RunEvent;
      } catch {
        return;
      }

      // Mesajsız olaylar yalnızca bitiş sinyalidir; satır olarak gösterilmez.
      if (e.message && !seen.current.has(e.seq)) {
        seen.current.add(e.seq);
        setEvents((prev) => [...prev, e]);
      }

      if (e.terminal) {
        if (e.status) setTerminalStatus(e.status);
        source.close();
        setConnected(false);
      }
    };

    source.onerror = () => {
      // EventSource kendiliğinden yeniden bağlanır; geçmiş baştan gelir ve
      // seq ile tekilleştirilir. Burada yalnızca göstergeyi düşürüyoruz.
      setConnected(false);
    };

    return () => {
      source.close();
      setConnected(false);
    };
  }, [runId, enabled]);

  return { events, terminalStatus, connected };
}
