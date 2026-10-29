import { http } from "./request";

export async function logoutCurrentSession(): Promise<void> {
    await http.post<{ loggedOut: boolean }>("/logout");
}
