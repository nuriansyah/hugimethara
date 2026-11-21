import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  poweredByHeader: false,
  // Workspace packages ship TypeScript source; Next compiles them with the app.
  transpilePackages: ["@hugimethara/ui", "@hugimethara/api-client", "@hugimethara/shared-types"],
  cacheComponents: true,
  partialPrefetching: true,
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
};

export default nextConfig;
