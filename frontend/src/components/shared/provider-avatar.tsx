// One letter in a soft accent circle for every provider/model row
// (DESIGN.md 6.2): consistent, no per-vendor trademark artwork to
// maintain.
export function ProviderAvatar({ name }: { name: string }) {
  return (
    <span className="mr-2 inline-flex size-[22px] flex-none items-center justify-center rounded-full bg-accent text-[9.5px] font-bold text-accent-foreground">
      {name.slice(0, 1).toUpperCase()}
    </span>
  )
}
