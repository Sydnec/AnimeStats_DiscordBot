import { SlashCommandBuilder } from 'discord.js';
import * as db from '../db.js';

export const data = new SlashCommandBuilder()
  .setName('follow')
  .setDescription('Suivre vos stats AniList en MP')
  .addStringOption(opt => opt.setName('username').setDescription('Votre pseudo AniList').setRequired(true))
  .addStringOption(opt => opt.setName('mask').setDescription("Masque 3 caractères 'dmy' où 1=set,0=unset,*=laisser tel quel, ex: 1*0").setRequired(true));

async function validateAniListUsername(username) {
  const query = `
    query ($userName: String) {
      MediaListCollection(userName: $userName, type: ANIME) {
        lists { entries { media { id } } }
      }
      User(name: $userName) { id }
    }
  `;
  const res = await fetch('https://graphql.anilist.co', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
    body: JSON.stringify({ query, variables: { userName: username } }),
  });
  const data = await res.json();
  // consider valid if User exists or MediaListCollection has lists
  if (data && (data.data && (data.data.User || (data.data.MediaListCollection && data.data.MediaListCollection.lists && data.data.MediaListCollection.lists.length > 0)))) {
    return true;
  }
  return false;
}

export async function execute(interaction) {
  const username = interaction.options.getString('username', true);
  const mask = interaction.options.getString('mask', true);

  // default freqs for this invocation are undefined (so we can merge). We'll compute final flags below.
  let daily = null, monthly = null, yearly = null;
  if (mask) {
    // expect mask length 3: d m y
    if (!/^[01\*]{3}$/.test(mask)) {
  await interaction.reply({ content: "Masque invalide. Utilisez 3 caractères parmi 0,1,* (ex: 1*0).", flags: 64 });
      return;
    }
    const [d,m,y] = mask.split('');
    // map: '1' -> set to 1, '0' -> set to 0, '*' -> keep existing (we'll fetch existing later)
    daily = d === '1' ? true : d === '0' ? false : null;
    monthly = m === '1' ? true : m === '0' ? false : null;
    yearly = y === '1' ? true : y === '0' ? false : null;
  }

  // validate AniList username
  let ok = false;
  try {
    ok = await validateAniListUsername(username);
  } catch (e) {
    console.error('AniList validation error', e);
  }
  if (!ok) {
  await interaction.reply({ content: 'Pseudo AniList invalide ou introuvable.', flags: 64 });
    return;
  }

  try {
    // If any of daily/monthly/yearly is null, treat it as 'keep existing' when merge=true in DB function.
    // For new users, null -> false (DB function will treat absence as 0 when writing).
    await db.addOrUpdateFollower(interaction.user.id, username, { daily, monthly, yearly }, true);
  await interaction.reply({ content: `Vous êtes suivi pour ${username} (daily:${daily}, monthly:${monthly}, yearly:${yearly}).`, flags: 64 });
  } catch (e) {
    console.error('follow command error', e);
  await interaction.reply({ content: 'Erreur lors de l enregistrement. Voir logs.', flags: 64 });
  }

}
