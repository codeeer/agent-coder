"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { ApiError } from "./api";

/**
 * TanStack Query sağlayıcısı.
 *
 * QueryClient useState içinde kurulur; her render'da yenisi üretilirse
 * önbellek sürekli sıfırlanırdı.
 */
export function QueryProvider({ children }: { children: React.ReactNode }) {
  const [client] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 30_000,
            retry: (failureCount, error) => {
              // İstemci hatalarını (geçersiz istek, bulunamadı) tekrar denemek
              // anlamsız; yalnızca ağ/sunucu sorunlarında tekrar dene.
              if (error instanceof ApiError && error.status >= 400 && error.status < 500) {
                return false;
              }
              return failureCount < 2;
            },
          },
          mutations: { retry: false },
        },
      }),
  );

  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}
