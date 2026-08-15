export function PageHeader({ title, action }: { title: string; action?: React.ReactNode }) {
  return (
    <div className="mb-4 flex items-center justify-between">
      <h1 className="text-[15px] font-semibold text-foreground">{title}</h1>
      {action}
    </div>
  )
}
