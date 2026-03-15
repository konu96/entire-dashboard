import { defineConfig } from "orval";

export default defineConfig({
  dashboard: {
    input: {
      target: "../api/openapi.yml",
    },
    output: {
      target: "src/api/generated.ts",
      client: "fetch",
      baseUrl: "http://localhost:8080",
      mode: "single",
    },
  },
});
