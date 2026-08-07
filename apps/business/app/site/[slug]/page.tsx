import { PublicSite } from "@/components/public-site";
export default async function Page({params}:{params:Promise<{slug:string}>}){const{slug}=await params;return <PublicSite slug={slug}/>}
