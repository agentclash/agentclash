import { WorkspaceReports } from "@/components/share/workspace-reports";
export const dynamic = "force-dynamic";
export default async function Page({params}:{params:Promise<{workspaceId:string}>}) {return <WorkspaceReports workspaceId={(await params).workspaceId}/>;}
