import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Docker imajını küçük tutar: sadece gereken node_modules kopyalanır.
  output: "standalone",
  reactStrictMode: true,
  typedRoutes: true,
};

export default nextConfig;
