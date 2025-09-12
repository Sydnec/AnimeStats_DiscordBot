import { SlashCommandBuilder } from 'discord.js';
import * as db from '../db.js';

export const data = new SlashCommandBuilder()
  .setName('unfollow')
  .setDescription('Arrêter de suivre vos stats');

export async function execute(interaction) {
  try {
    const res = db.removeFollower(interaction.user.id);
  await interaction.reply({ content: 'Vous ne recevez plus les récaps.', flags: 64 });
  } catch (e) {
    console.error('unfollow command error', e);
  await interaction.reply({ content: 'Erreur lors de la suppression. Voir logs.', flags: 64 });
  }
}
