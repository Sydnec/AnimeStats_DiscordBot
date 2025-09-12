import assert from 'assert';
import { computeStatsFromAniListData } from '../src/lib/stats.js';

// Build sample AniList-like data
const nowSec = Math.floor(Date.now() / 1000);
const oneDay = 24*60*60;
const data = {
  data: {
    MediaListCollection: {
      lists: [
        {
          entries: [
            { media: { title: { romaji: 'A' }, duration: 24 }, progress: 3, updatedAt: nowSec - oneDay },
            { media: { title: { romaji: 'B' }, duration: 24 }, progress: 2, updatedAt: nowSec - 2*oneDay },
            { media: { title: { romaji: 'A' }, duration: 24 }, progress: 1, updatedAt: nowSec - oneDay },
          ]
        }
      ]
    }
  }
};

const startDate = new Date((nowSec - 3*oneDay) * 1000);
const endDate = new Date(nowSec * 1000);
const stats = computeStatsFromAniListData(data, startDate, endDate);
assert.strictEqual(stats.totalEpisodes, 6);
assert.strictEqual(stats.totalMinutes, 6*24);
console.log('stats tests passed');
