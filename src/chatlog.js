// Lenwy Was Here: Lenwy Whatsmeow Chat Log

import chalk from "chalk";

export function chatLog(msg) {
  const { type = "Chat", timestamp, sender, pushName, body, isFromMe } = msg;

  if (isFromMe) return; // Lewati Pesan Diri Sendiri

  // Format Waktu (HH:mm)
  const date = timestamp ? new Date(timestamp * 1000) : new Date();
  const timeStr = date.toLocaleTimeString("id-ID", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });

  // Pewarnaan Badge
  const typeBadge = chalk.yellow.bold(`[${type}]`);
  const timeBadge = chalk.blue.bold(`[${timeStr}]`);
  // const senderNumber = chalk.yellow(`[${sender || "Unknown"}]`);
  const nameStr = chalk.green.bold(pushName || "No Name");
  const contentStr = chalk.white(body || "");

  console.log(`${typeBadge} ${timeBadge} ${nameStr} : ${contentStr}`);
}
