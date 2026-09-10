// Lenwy Was Here: Lenwy Whatsmeow Utilities

export function extractNumber(jid = "") {
  if (typeof jid !== "string") return "";
  return jid.split("@")[0].split(":")[0].replace(/\D/g, "");
}

export function isLid(jid = "") {
  return typeof jid === "string" && jid.endsWith("@lid");
}

export function isGroup(jid = "") {
  return typeof jid === "string" && jid.endsWith("@g.us");
}

export function jidNormalizedUser(jid = "") {
  if (typeof jid !== "string") return "";
  if (isGroup(jid)) return jid;

  const [userAndDevice, domain] = jid.split("@");
  if (!userAndDevice) return jid;

  const user = userAndDevice.split(":")[0];
  return domain ? `${user}@${domain}` : user;
}
