// Utilities pour interroger AniList (userId resolution + pagination des activities)
export async function fetchAniListUserId(username) {
  const query = `
    query ($name: String) {
      User(name: $name) {
        id
      }
    }
  `;
  const variables = { name: username };
  const res = await fetch("https://graphql.anilist.co", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
    },
    body: JSON.stringify({ query, variables }),
  });
  const data = await res.json();
  return data.data?.User?.id;
}

// Simple in-memory TTL cache: key -> { expiresAt: number, value }
const cache = new Map();

export async function fetchAniListActivitiesPaginated(username, perPage = 100, maxPages = null) {
  const envMax = parseInt(process.env.ANILIST_MAX_PAGES || '', 10);
  const finalMaxPages = Number.isFinite(envMax) && envMax > 0 ? envMax : (Number.isFinite(maxPages) && maxPages > 0 ? maxPages : 10);
  const cacheTtlSec = parseInt(process.env.ANILIST_CACHE_TTL_SECONDS || '', 10) || 0;

  const cacheKey = `activities:${username}:${perPage}:${finalMaxPages}`;
  const now = Date.now();
  if (cacheTtlSec > 0 && cache.has(cacheKey)) {
    const entry = cache.get(cacheKey);
    if (entry.expiresAt > now) {
      return entry.value;
    } else {
      cache.delete(cacheKey);
    }
  }

  // Résout l'userId puis pagine Page(perPage,page) pour récupérer toutes les activities
  const userId = await fetchAniListUserId(username);
  if (!userId) throw new Error('Utilisateur AniList introuvable');

  const allActivities = [];
  for (let page = 1; page <= finalMaxPages; page++) {
    const query = `
      query ($userId: Int, $page: Int, $perPage: Int) {
        Page(page: $page, perPage: $perPage) {
          pageInfo { hasNextPage }
          activities(userId: $userId, type: ANIME_LIST) {
            ... on ListActivity {
              createdAt
              progress
              media {
                title { romaji english native }
                duration
                id
              }
            }
          }
        }
      }
    `;
    const variables = { userId, page, perPage };
    const res = await fetch('https://graphql.anilist.co', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify({ query, variables }),
    });
    const json = await res.json();
    const activities = (json && json.data && json.data.Page && json.data.Page.activities) || [];
    allActivities.push(...activities);
    const hasNext = json && json.data && json.data.Page && json.data.Page.pageInfo && json.data.Page.pageInfo.hasNextPage;
    if (!hasNext) break;
  }

  const result = { data: { Page: { activities: allActivities } } };
  if (cacheTtlSec > 0) {
    cache.set(cacheKey, { expiresAt: now + cacheTtlSec * 1000, value: result });
  }

  return result;
}

export default { fetchAniListUserId, fetchAniListActivitiesPaginated };
