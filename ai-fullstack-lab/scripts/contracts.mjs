import { spawnSync } from "node:child_process";
import { readFileSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { resolve, join } from "node:path";
import { fileURLToPath } from "node:url";
const root = fileURLToPath(new URL("../", import.meta.url));
const check = process.argv[2] === "check";
const temp = mkdtempSync(join(tmpdir(), "lab-contracts-"));
function run(command, args, cwd = root) {
  const result = spawnSync(command, args, { cwd, stdio: "inherit" });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`${command} failed: ${result.status}`);
}
try {
  const ts = "apps/web/src/lib/api.generated.d.ts";
  const go = "apps/api/internal/api/generated.go";
  const tsOut = check ? join(temp, "api.d.ts") : resolve(root, ts);
  const goOut = check ? join(temp, "generated.go") : resolve(root, go);
  run(resolve(root, "node_modules/.bin/openapi-typescript"), ["contracts/openapi.yaml", "-o", tsOut]);
  run("go", ["run", "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1", "-config", "oapi-codegen.yaml", "-o", goOut, "../../contracts/openapi.yaml"], resolve(root, "apps/api"));
  if (check) {
    for (const [file, generated] of [[ts, tsOut], [go, goOut]]) {
      if (!readFileSync(resolve(root, file)).equals(readFileSync(generated))) {
        throw new Error(`${file} 已漂移；运行 npm run generate 并提交生成文件`);
      }
    }
    console.log("契约生成结果一致");
  }
} finally { rmSync(temp, { recursive: true, force: true }); }
