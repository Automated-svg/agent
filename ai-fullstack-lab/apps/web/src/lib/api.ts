import createClient from "openapi-fetch";
import type { paths, components } from "./api.generated";
export type Task = components["schemas"]["Task"];
export type TaskStatus = components["schemas"]["TaskStatus"];
export const api = createClient<paths>({ baseUrl: "/" });
