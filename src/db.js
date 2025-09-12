import Database from 'better-sqlite3';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const dbPath = path.join(__dirname, '..', 'data', 'animestats.db');

const db = new Database(dbPath);

// Table: followers
// user_id (Discord) - TEXT PRIMARY KEY
// anilist_username - TEXT
// freq_daily - INTEGER (0/1)
// freq_monthly - INTEGER (0/1)
// freq_yearly - INTEGER (0/1)

db.exec(`
CREATE TABLE IF NOT EXISTS followers (
  user_id TEXT PRIMARY KEY,
  anilist_username TEXT NOT NULL,
  freq_daily INTEGER DEFAULT 0,
  freq_monthly INTEGER DEFAULT 0,
  freq_yearly INTEGER DEFAULT 0,
  created_at INTEGER DEFAULT (strftime('%s','now'))
);
`);

export function addOrUpdateFollower(userId, anilistUsername, freqs, merge = false) {
  const uid = String(userId);
  const desired = {
    freq_daily: freqs.daily ? 1 : 0,
    freq_monthly: freqs.monthly ? 1 : 0,
    freq_yearly: freqs.yearly ? 1 : 0,
  };
  if (merge) {
    const existing = getFollower(uid);
    if (existing) {
      // OR the flags so we only add new frequencies
      desired.freq_daily = existing.freq_daily || desired.freq_daily;
      desired.freq_monthly = existing.freq_monthly || desired.freq_monthly;
      desired.freq_yearly = existing.freq_yearly || desired.freq_yearly;
    }
  }
  const stmt = db.prepare(`INSERT INTO followers (user_id, anilist_username, freq_daily, freq_monthly, freq_yearly)
    VALUES (@user_id, @anilist_username, @freq_daily, @freq_monthly, @freq_yearly)
    ON CONFLICT(user_id) DO UPDATE SET
      anilist_username=excluded.anilist_username,
      freq_daily=excluded.freq_daily,
      freq_monthly=excluded.freq_monthly,
      freq_yearly=excluded.freq_yearly
  `);
  const info = stmt.run({
    user_id: uid,
    anilist_username: anilistUsername,
    freq_daily: desired.freq_daily,
    freq_monthly: desired.freq_monthly,
    freq_yearly: desired.freq_yearly,
  });
  return info;
}

export function removeFollower(userId) {
  const stmt = db.prepare('DELETE FROM followers WHERE user_id = ?');
  return stmt.run(String(userId));
}

export function getFollower(userId) {
  const stmt = db.prepare('SELECT * FROM followers WHERE user_id = ?');
  return stmt.get(String(userId));
}

export function listFollowersByFrequency(freq) {
  // freq is 'daily' | 'monthly' | 'yearly'
  const col = freq === 'daily' ? 'freq_daily' : freq === 'monthly' ? 'freq_monthly' : 'freq_yearly';
  const stmt = db.prepare(`SELECT * FROM followers WHERE ${col} = 1`);
  return stmt.all();
}

export function listAllFollowers() {
  const stmt = db.prepare('SELECT * FROM followers');
  return stmt.all();
}
