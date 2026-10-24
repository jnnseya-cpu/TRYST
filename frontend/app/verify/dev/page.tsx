import { notFound } from "next/navigation";
import DevVerify from "./DevVerify";

// The stand-in for the provider's hosted check exists only on dev stacks (TRYST_ENV=dev);
// everywhere else this address is a 404, like the API route behind it.
export const dynamic = "force-dynamic";

export default function Page() {
  if (process.env.TRYST_ENV !== "dev") notFound();
  return <DevVerify />;
}
