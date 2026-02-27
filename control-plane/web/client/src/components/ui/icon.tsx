import { cn } from "../../lib/utils";
import {
  SquaresFour,
  Stack,
  Cpu,
  Play,
  FlowArrow,
  Settings,
  UserCircle,
  GridFour,
  Package,
  Pulse,
  Sun,
  Moon,
  Monitor,
  ShieldCheck,
  Identification,
  FileText,
  GithubLogo,
  Question,
  Bot,
  Users,
  Link,
  RadioTower,
  Launch,
  ShareNetwork,
  CurrencyDollar,
  Wallet,
  Storefront,
  ShoppingCart,
  HandCoins,
  Receipt,
  Coins,
} from "@/components/ui/icon-bridge";
import type { IconComponent, IconWeight } from "@/components/ui/icon-bridge";

const icons = {
  activity: Pulse,
  dashboard: SquaresFour,
  "data-center": Stack,
  function: Cpu,
  run: Play,
  "flow-data": FlowArrow,
  settings: Settings,
  user: UserCircle,
  grid: GridFour,
  package: Package,
  sun: Sun,
  moon: Moon,
  monitor: Monitor,
  "shield-check": ShieldCheck,
  identification: Identification,
  documentation: FileText,
  github: GithubLogo,
  support: Question,
  bot: Bot,
  users: Users,
  link: Link,
  broadcast: RadioTower,
  rocket: Launch,
  network: ShareNetwork,
  wallet: Wallet,
  'currency-dollar': CurrencyDollar,
  storefront: Storefront,
  'shopping-cart': ShoppingCart,
  'hand-coins': HandCoins,
  receipt: Receipt,
  coins: Coins,
} as const;

export interface IconProps {
  name: keyof typeof icons;
  className?: string;
  size?: number;
  weight?: IconWeight;
}

export function Icon({ name, className, size = 16, weight = "regular" }: IconProps) {
  const IconComponent = icons[name] as IconComponent;

  if (!IconComponent) {
    console.warn(`Icon "${name}" not found`);
    return null;
  }

  return (
    <IconComponent
      className={cn("shrink-0", className)}
      size={size}
      weight={weight}
    />
  );
}
