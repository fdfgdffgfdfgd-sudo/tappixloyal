import { JoinPortal } from "@/components/customer-portal";
export default async function Page({params}:{params:Promise<{token:string}>}){const{token}=await params;return <JoinPortal token={token}/>}
