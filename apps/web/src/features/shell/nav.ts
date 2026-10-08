import {
  Download,
  Inbox,
  LayoutGrid,
  ListChecks,
  Megaphone,
  Settings2,
  Upload,
  type LucideIcon,
} from "lucide-react";

export type Role = "seller" | "reviewer";
export type NavKey =
  "upload" | "products" | "brands" | "received" | "review" | "export" | "channels";

export interface NavItem {
  key: NavKey;
  label: string;
  screen: string;
  icon: LucideIcon;
  reviewerOnly?: boolean;
  sellerOnly?: boolean;
}

// Each role sees only what it acts on (ADR-0012). A seller uploads, fixes,
// sets the brand voice and collects the files a reviewer sent; a reviewer
// checks, approves, exports and sends, and owns the channel rules.
export const NAV: NavItem[] = [
  { key: "upload", label: "Upload", screen: "S-03", icon: Upload, sellerOnly: true },
  { key: "products", label: "Products", screen: "S-04", icon: LayoutGrid },
  { key: "brands", label: "Brand voice", screen: "S-08", icon: Megaphone, sellerOnly: true },
  { key: "received", label: "Received files", screen: "S-09", icon: Inbox, sellerOnly: true },
  { key: "review", label: "Review", screen: "S-05", icon: ListChecks, reviewerOnly: true },
  { key: "export", label: "Export", screen: "S-06", icon: Download, reviewerOnly: true },
  { key: "channels", label: "Channels", screen: "S-07", icon: Settings2, reviewerOnly: true },
];

/** navAllowed says whether a role has a page in its navigation. */
export function navAllowed(role: Role, key: NavKey): boolean {
  const item = NAV.find((n) => n.key === key);
  if (!item) return false;
  return role === "reviewer" ? !item.sellerOnly : !item.reviewerOnly;
}
