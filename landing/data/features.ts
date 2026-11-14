import {
  mdiCodeBracesBox,
  mdiDownloadCircleOutline,
  mdiMessageTextOutline,
  mdiRobotOutline,
  mdiSyncCircle,
  mdiViewDashboardOutline
} from "@mdi/js";

export const features = [
  { id: "oneRepo", icon: mdiRobotOutline, accent: "#00f0ff" },
  { id: "growOutputs", icon: mdiViewDashboardOutline, accent: "#ff00ff" },
  { id: "validate", icon: mdiCodeBracesBox, accent: "#39ff14" },
  { id: "clearBoundary", icon: mdiMessageTextOutline, accent: "#ffd700" },
  { id: "installAcrossAgents", icon: mdiDownloadCircleOutline, accent: "#00f0ff" },
  { id: "managedLifecycle", icon: mdiSyncCircle, accent: "#ff00ff" }
] as const;
