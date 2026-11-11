const incoming = new URLSearchParams(location.hash.slice(1)).get("token");
if (incoming) {
  sessionStorage.setItem("agent-desk-token", incoming);
  history.replaceState(null, "", location.pathname);
}
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch("/api/" + path, {
    ...init,
    headers: {
      Authorization: "Bearer " + sessionStorage.getItem("agent-desk-token"),
      "Content-Type": "application/json",
      ...init.headers,
    },
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Request failed");
  return data;
}
export const query = (obj: Record<string, string>) =>
  new URLSearchParams(obj).toString();
