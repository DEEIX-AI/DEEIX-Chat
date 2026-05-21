export type SidebarUser = {
  name: string;
  email: string;
  avatar: string;
  role?: string;
};

export type SidebarLink = {
  title: string;
  url: string;
};

export type SidebarData = {
  user?: SidebarUser;
  recents: SidebarLink[];
};
