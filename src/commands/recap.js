import { SlashCommandBuilder } from 'discord.js';
import * as db from '../db.js';

export const data = new SlashCommandBuilder()
  .setName('recap')
  .setDescription('Demander un récap des X derniers jours (pour test)')
  .addIntegerOption(opt => opt.setName('days').setDescription('Nombre de jours à couvrir').setRequired(true))
  .addStringOption(opt => opt.setName('username').setDescription('Pseudo AniList (optionnel)'));

export async function execute(interaction) {
  const days = interaction.options.getInteger('days', true);
  let username = interaction.options.getString('username') || null;
  if (days <= 0 || days > 365) {
  await interaction.reply({ content: 'Nombre de jours invalide (1-365)', flags: 64 });
    return;
  }

  // send an immediate recap using sendStatsForUser via dynamic import of index
  try {
    // if username not provided, try to read from DB for this user
    if (!username) {
      const row = await db.getFollower(interaction.user.id);
      if (row && row.anilist_username) {
        username = row.anilist_username;
      } else {
  await interaction.reply({ content: "Aucun AniList associé à votre compte. Utilisez /follow <pseudo> <mask> pour vous abonner.", flags: 64 });
        return;
      }
    }
    // import the sendStatsForUser function from index.js
    const mod = await import('../index.js');
    // prefer sendStatsForUserWithDays if available
  await interaction.reply({ content: `Envoi d'un récap pour les ${days} derniers jours...`, flags: 64 });
    if (typeof mod.sendStatsForUserWithDays === 'function') {
      await mod.sendStatsForUserWithDays(days, interaction.user.id, username);
    } else if (typeof mod.sendStatsForUser === 'function') {
      // fallback: if sendStatsForUser exists, only exact 30 day rolling is supported
      if (days === 30) {
        await mod.sendStatsForUser('rolling', interaction.user.id, username);
      } else {
  await interaction.followUp({ content: 'Fonction de récap pour une période personnalisée indisponible sur ce déploiement.', flags: 64 });
      }
    } else {
  await interaction.followUp({ content: 'Fonction de récap non disponible.', flags: 64 });
    }
    } catch (e) {
    logger.error('recap command error', e);
  await interaction.reply({ content: 'Erreur lors de la requête de récap.', flags: 64 });
  }
}
