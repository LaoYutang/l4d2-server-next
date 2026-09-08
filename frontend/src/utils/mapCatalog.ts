import { officialMaps } from '../data/officialMaps';
import type { MapMissionCampaign, MapMissionChapter } from '../services/api';

export interface MapCatalogCampaign {
  Title: string;
  Chapters: MapMissionChapter[];
  VpkName: string | null;
  IsCustom: boolean;
}

const isCampaign = (value: unknown): value is MapMissionCampaign => {
  if (!value || typeof value !== 'object') return false;
  const campaign = value as Partial<MapMissionCampaign>;
  return typeof campaign.Title === 'string' && Array.isArray(campaign.Chapters);
};

const getServerCampaigns = (serverMaps: unknown): MapMissionCampaign[] => {
  if (Array.isArray(serverMaps)) return serverMaps.filter(isCampaign);
  if (serverMaps && typeof serverMaps === 'object') {
    const campaigns = (serverMaps as { campaigns?: unknown }).campaigns;
    if (Array.isArray(campaigns)) return campaigns.filter(isCampaign);
  }
  return [];
};

export const buildMapCatalog = (serverMaps: unknown): MapCatalogCampaign[] => {
  const maps: MapCatalogCampaign[] = officialMaps.map((campaign) => ({
    Title: campaign.Title,
    Chapters: campaign.Chapters,
    IsCustom: false,
    VpkName: null,
  }));

  for (const serverCampaign of getServerCampaigns(serverMaps)) {
    const isOfficialCampaign = officialMaps.some((officialCampaign) =>
      serverCampaign.Chapters.some((serverChapter) =>
        officialCampaign.Chapters.some(
          (officialChapter) => officialChapter.Code === serverChapter.Code
        )
      )
    );

    if (!isOfficialCampaign) {
      maps.push({
        Title: serverCampaign.Title || 'Unknown Campaign',
        Chapters: serverCampaign.Chapters || [],
        IsCustom: true,
        VpkName: serverCampaign.VpkName || null,
      });
    }
  }

  return maps;
};
