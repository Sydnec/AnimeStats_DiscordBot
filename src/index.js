import { fetchAniListActivitiesPaginated } from './lib/anilist.js';
import { Client, GatewayIntentBits, REST, Routes } from "discord.js";
import { logger } from "./lib/logger.js";
import { computeStatsFromAniListData } from "./lib/stats.js";
import cron from "node-cron";
import dotenv from "dotenv";
import * as db from "./db.js";
import fs from "fs/promises";
import path from "path";
import { fileURLToPath } from "url";
import { pathToFileURL } from "url";

dotenv.config();

// --- CONFIGURATION ---
const DISCORD_TOKEN = process.env.DISCORD_TOKEN;

if (!DISCORD_TOKEN) {
  logger.error("Missing config in .env (DISCORD_TOKEN required)");
  process.exit(1);
}

const client = new Client({ intents: [GatewayIntentBits.Guilds] });
// map command name -> module
const commandsMap = new Map();

// Helper: format minutes as '6h12min' (round minutes). If exactly 0 minutes -> '0h'
function formatMinutes(mins) {
  const total = Math.round(Number(mins) || 0);
  const h = Math.floor(total / 60);
  const m = total % 60;
  if (m === 0) return `${h}h`;
  return `${h}h${m}min`;
}

// Fonction principale : envoie les stats
async function sendStatsForUser(
  period = "month",
  discordUserId,
  anilistUsername
) {
  try {
    // fetch using the provided AniList username if available, else fallback to global

  if (!anilistUsername) throw new Error("AniList username not provided");
  const data = await fetchAniListActivitiesPaginated(anilistUsername);

    const now = new Date();
    let startDate, endDate;
    let title = "";

    if (period === "month") {
      const year = now.getFullYear();
      const month = now.getMonth();
      // Stats du mois précédent
      startDate = new Date(year, month - 1, 1);
      endDate = new Date(year, month, 0, 23, 59, 59, 999);
      title = `📊 Stats AniList - ${startDate.toLocaleString("fr-FR", {
        month: "long",
        year: "numeric",
      })}`;
    } else if (period === "year") {
      startDate = new Date(now.getFullYear() - 1, 0, 1);
      endDate = new Date(now.getFullYear() - 1, 11, 31, 23, 59, 59, 999);
      title = `📆 Stats AniList - Année ${startDate.getFullYear()}`;
    } else if (period === "rolling") {
      // Mois glissant = derniers 30 jours
      endDate = now;
      startDate = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000);
      title = `📈 Récapitulatif - ${new Intl.DateTimeFormat("fr-FR", {
        dateStyle: "medium",
      }).format(startDate)} → ${new Intl.DateTimeFormat("fr-FR", {
        dateStyle: "medium",
      }).format(endDate)}`;
    } else if (period === "day") {
      // Récapitulatif journalier : jour précédent (00:00 → 23:59:59)
      const yesterday = new Date(now);
      yesterday.setDate(now.getDate() - 1);
      startDate = new Date(
        yesterday.getFullYear(),
        yesterday.getMonth(),
        yesterday.getDate(),
        0,
        0,
        0,
        0
      );
      endDate = new Date(
        yesterday.getFullYear(),
        yesterday.getMonth(),
        yesterday.getDate(),
        23,
        59,
        59,
        999
      );
      title = `📅 Récap journalier - ${new Intl.DateTimeFormat("fr-FR", {
        dateStyle: "medium",
      }).format(startDate)}`;
    }

    const { totalMinutes, totalEpisodes, dailyData, topDay, titles } =
      computeStatsFromAniListData(data, startDate, endDate);

    // Calcul de la moyenne quotidienne en minutes (sur la période considérée)
    // nombre de jours inclus dans la période (inclusif de startDate et endDate)
  const msPerDay = 24 * 60 * 60 * 1000;
  const daysCount = Math.max(1, Math.ceil((endDate.getTime() - startDate.getTime()) / msPerDay));
    const avgMinutesPerDay = totalMinutes / daysCount;

    // Préparer le texte récapitulatif (texte uniquement, pas de fichier)
    let titlesList;
    let content;
    const totalTimeText = formatMinutes(totalMinutes);
    const avgTimeText = `${formatMinutes(avgMinutesPerDay)}/j`;
      if (!titles || titles.length === 0) {
      titlesList = "Aucun anime enregistré";
      content = `${title}\n⏱️ Temps total : **${totalTimeText}**\n⏳ Moyenne journalière : **${avgTimeText}**\n🎬 Épisodes regardés : **${totalEpisodes}**\n\n${titlesList}`;
    } else {
      const sorted = titles.slice().sort((a, b) => b.count - a.count);
      let maxAnimes = 10;
      let valid = false;
      do {
        const truncated = sorted.slice(0, maxAnimes);
        titlesList = `# Animes :\n${truncated.map((t) => `- ${t.title} (${t.count})`).join("\n")}`;
        if (sorted.length > maxAnimes) {
          titlesList += `\n...et ${sorted.length - maxAnimes} autres`;
        }
        let topDayText = topDay
          ? `${new Intl.DateTimeFormat("fr-FR", { dateStyle: "medium" }).format(new Date(topDay))} (${formatMinutes(dailyData[topDay])})`
          : "N/A";
        if (period === "day") {
          content = `${title}\n⏱️ Temps total : **${totalTimeText}**\n⏳ Moyenne journalière : **${avgTimeText}**\n🎬 Épisodes regardés : **${totalEpisodes}**\n\n${titlesList}`;
        } else {
          content = `${title}\n⏱️ Temps total : **${totalTimeText}**\n⏳ Moyenne journalière : **${avgTimeText}**\n🎬 Épisodes regardés : **${totalEpisodes}**\n🔥 Jour le plus actif : **${topDayText}**\n\n${titlesList}`;
        }
        if (content.length <= 4000) valid = true;
        else maxAnimes--;
      } while (!valid && maxAnimes > 0);
    }

    // Send DM to the user
    const user = await client.users.fetch(String(discordUserId));
    await user.send({ content });

    // pas de fichier à nettoyer
  } catch (err) {
  logger.error("Erreur lors de l'envoi des stats :", err);
  }
}

// helper: send stats for the last N days
export async function sendStatsForUserWithDays(days, discordUserId, anilistUsername) {
  try {
    const now = new Date();
    const endDate = now;
    const startDate = new Date(now.getTime() - days * 24 * 60 * 60 * 1000);

  if (!anilistUsername) throw new Error("AniList username not provided");
  const data = await fetchAniListActivitiesPaginated(anilistUsername);
    const { totalMinutes, totalEpisodes, dailyData, topDay, titles } =
      computeStatsFromAniListData(data, startDate, endDate);

    // Calcul de la moyenne journalière pour la période fournie
  const msPerDay = 24 * 60 * 60 * 1000;
  const daysCount = Math.max(1, Math.ceil((endDate.getTime() - startDate.getTime()) / msPerDay));
    const avgMinutesPerDay = totalMinutes / daysCount;

    let titlesList;
    let content;
    const totalTimeText = formatMinutes(totalMinutes);
    const avgTimeText = `${formatMinutes(avgMinutesPerDay)}/j`;
    if (!titles || titles.length === 0) {
      titlesList = "Aucun anime enregistré";
      content = `📈 Récapitulatif - ${new Intl.DateTimeFormat("fr-FR", {
        dateStyle: "medium",
      }).format(startDate)} → ${new Intl.DateTimeFormat("fr-FR", {
        dateStyle: "medium",
      }).format(endDate)}\n⏱️ Temps total : **${totalTimeText}**\n⏳ Moyenne journalière : **${avgTimeText}**\n🎬 Épisodes regardés : **${totalEpisodes}**\n\n${titlesList}`;
    } else {
      const sorted = titles.slice().sort((a, b) => b.count - a.count);
      let maxAnimes = 10;
      let valid = false;
      do {
        const truncated = sorted.slice(0, maxAnimes);
        titlesList = `# Animes :\n${truncated.map((t) => `- ${t.title} (${t.count})`).join("\n")}`;
        if (sorted.length > maxAnimes) {
          titlesList += `\n...et ${sorted.length - maxAnimes} autres`;
        }
        content = `📈 Récapitulatif - ${new Intl.DateTimeFormat("fr-FR", {
          dateStyle: "medium",
        }).format(startDate)} → ${new Intl.DateTimeFormat("fr-FR", {
          dateStyle: "medium",
        }).format(endDate)}\n⏱️ Temps total : **${totalTimeText}**\n⏳ Moyenne journalière : **${avgTimeText}**\n🎬 Épisodes regardés : **${totalEpisodes}**\n`;
        if (days > 1) {
          const topDayText = topDay
            ? `${new Intl.DateTimeFormat("fr-FR", { dateStyle: "medium" }).format(new Date(topDay))} (${formatMinutes(dailyData[topDay])})`
            : "N/A";
          content += `🔥 Jour le plus actif : **${topDayText}**\n`;
        }
        content += `\n${titlesList}`;
        if (content.length <= 4000) valid = true;
        else maxAnimes--;
      } while (!valid && maxAnimes > 0);
    }

    const user = await client.users.fetch(String(discordUserId));
    await user.send({ content });
  } catch (e) {
  logger.error("sendStatsForUserWithDays error", e);
    throw e;
  }
}

// Événement prêt
client.once("clientReady", () => {
  logger.info(`✅ Connecté en tant que ${client.user.tag}`);
  // Load command modules from src/commands dynamically
  const rest = new REST({ version: "10" }).setToken(DISCORD_TOKEN);
  const commandsDir = path.join(
    path.dirname(fileURLToPath(import.meta.url)),
    "commands"
  );
  const commandModules = [];
  (async () => {
    try {
      const files = await fs.readdir(commandsDir);
      for (const f of files) {
        if (!f.endsWith(".js")) continue;
        try {
          // ensure we import using a file:// URL so ESM resolver treats it as a local file
          const fullPath = path.join(commandsDir, f);
          const mod = await import(pathToFileURL(fullPath).href);
          // only keep modules that export both `data` and `execute`
          const hasData = !!mod && !!mod.data;
          const hasExecute = !!mod && typeof mod.execute === "function";
          if (hasData && hasExecute) {
            commandModules.push(mod);
            // determine command name
            let name = null;
            if (typeof mod.data.name === "string") {
              name = mod.data.name;
            } else if (mod.data && typeof mod.data.toJSON === "function") {
              const json = mod.data.toJSON();
              if (json && json.name) name = json.name;
            }
            if (name) {
              commandsMap.set(name, mod);
              logger.info(`Loaded command: ${name} (file: ${f})`);
            } else {
              // If no name could be determined, still log filename for debug
              logger.warn(
                `Loaded command module without name property (file: ${f})`
              );
            }
          } else {
            logger.warn(`Skipping ${f}: missing data or execute export`);
          }
        } catch (e) {
          logger.error("Error importing command", f, e);
        }
      }
      const commands = commandModules.map((m) => m.data.toJSON());
      await rest.put(Routes.applicationCommands(client.user.id), {
        body: commands,
      });
  logger.info("Slash commands registered (dynamic)");
    } catch (e) {
  logger.error("Failed loading commands directory", e);
    }
  })();

  // Cron jobs: iterate DB and send per-frequency

  // monthly on 1st at 10:00 -> send 'month'
  cron.schedule("0 10 1 * *", async () => {
    const rows = db.listFollowersByFrequency("monthly");
    for (const row of rows) {
      try {
        await sendStatsForUser("month", row.user_id, row.anilist_username);
      } catch (e) {
  logger.error("Failed sending month to", row.user_id, e);
      }
    }
  });

  // yearly on Jan 1 at 12:00 -> send 'year'
  cron.schedule("0 12 1 1 *", async () => {
    const rows = db.listFollowersByFrequency("yearly");
    for (const row of rows) {
      try {
        await sendStatsForUser("year", row.user_id, row.anilist_username);
      } catch (e) {
  logger.error("Failed sending year to", row.user_id, e);
      }
    }
  });

  // No startup sends — stats will be sent only by cron jobs
});

// Interaction handling (slash commands)
client.on("interactionCreate", async (interaction) => {
  if (!interaction.isChatInputCommand()) return;
  const name = interaction.commandName;
  try {
    const cmd = commandsMap.get(name);
    if (!cmd || typeof cmd.execute !== "function") {
      await interaction.reply({
        content: "Commande non trouvée.",
        flags: 64,
      });
      return;
    }
    await cmd.execute(interaction);
  } catch (e) {
  logger.error("interaction handler error", e);
    if (interaction.replied || interaction.deferred) {
      try {
        await interaction.followUp({
          content: "Erreur interne.",
          flags: 64,
        });
      } catch (_) {}
    } else {
      try {
        await interaction.reply({
          content: "Erreur interne.",
          flags: 64,
        });
      } catch (_) {}
    }
  }
});

// Lancement du bot
client.login(DISCORD_TOKEN);

// ... AniList fetching is handled by `src/lib/anilist.js`
