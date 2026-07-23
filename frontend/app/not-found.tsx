import Link from "next/link";
import { ArrowRight } from "@phosphor-icons/react/dist/ssr";

export default function NotFound() {
  return (
    <div className="shell">
      <section className="notfound">
        <p className="label label-acid bracket">404 · no receipt found</p>
        <h1 className="d1">This drop is empty.</h1>
        <p className="lede">
          The page may have moved. Your source directory is still untouched.
        </p>
        <Link className="btn" href="/">
          Return home
          <ArrowRight size={14} weight="bold" />
        </Link>
      </section>
    </div>
  );
}
