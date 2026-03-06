import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  env: {
    API_GATEWAY_URL: process.env.API_GATEWAY_URL || "http://api-gateway",
    AUTH_SERVICE_URL: process.env.AUTH_SERVICE_URL || "http://auth-service",
  },
  async rewrites() {
    const apiGatewayUrl = process.env.API_GATEWAY_URL || "http://api-gateway";
    const authServiceUrl = process.env.AUTH_SERVICE_URL || "http://auth-service";
    return [
      {
        source: "/api/:path*",
        destination: `${apiGatewayUrl}/api/:path*`,
      },
      {
        source: "/auth/:path*",
        destination: `${authServiceUrl}/:path*`,
      },
    ];
  },
};

export default nextConfig;
