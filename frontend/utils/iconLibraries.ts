export type LibrarySort = "updated_desc" | "created_desc" | "published_desc" | "name_asc";

export type LibrarySortable = {
  id: number;
  name: string;
  createdAt?: string;
  updatedAt?: string;
  publishedAt?: string | null;
};

export function sortLibraries<T extends LibrarySortable>(list: T[], sort: LibrarySort): T[] {
  return [...list].sort((a, b) => {
    switch (sort) {
      case "name_asc":
        return a.name.localeCompare(b.name) || b.id - a.id;
      case "created_desc":
        return (b.createdAt || "").localeCompare(a.createdAt || "") || b.id - a.id;
      case "published_desc": {
        const ap = a.publishedAt || "";
        const bp = b.publishedAt || "";
        if (ap && bp) return bp.localeCompare(ap) || b.id - a.id;
        if (ap) return -1;
        if (bp) return 1;
        return (b.updatedAt || "").localeCompare(a.updatedAt || "") || b.id - a.id;
      }
      case "updated_desc":
      default:
        return (b.updatedAt || "").localeCompare(a.updatedAt || "") || b.id - a.id;
    }
  });
}
