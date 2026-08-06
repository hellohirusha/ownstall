// The directory groups stalls by exact category string, so this is a fixed
// list rather than free text. Left open, "Ceramics", "ceramics" and
// "Ceramics & Pottery" would each become their own filter chip and the
// category facet would be useless.
//
// Adding a value here is safe. Renaming one orphans the stalls already using
// the old string — migrate them in the same change.
export const STALL_CATEGORIES = [
  "Art & prints",
  "Beauty & wellness",
  "Books & zines",
  "Ceramics & pottery",
  "Clothing & accessories",
  "Craft supplies",
  "Digital & downloads",
  "Food & drink",
  "Home & living",
  "Jewellery",
  "Music & audio",
  "Plants & garden",
  "Stationery & paper",
  "Toys & games",
  "Vintage & second-hand",
  "Other",
] as const;

export type StallCategory = (typeof STALL_CATEGORIES)[number];
