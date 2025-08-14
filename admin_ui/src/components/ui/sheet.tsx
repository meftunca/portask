import * as React from "react"
import { cn } from "@/lib/utils"

interface SheetProps {
  children: React.ReactNode
  className?: string
}

const Sheet = ({ children, className }: SheetProps) => {
  return (
    <div
      className={cn(
        "fixed inset-0 z-50 bg-background/80 backdrop-blur-sm",
        className
      )}
    >
      {children}
    </div>
  )
}

const SheetContent = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement> & {
    side?: "top" | "bottom" | "left" | "right"
  }
>(({ className, side = "right", ...props }, ref) => {
  const sideClasses = {
    top: "top-0 border-b w-full",
    bottom: "bottom-0 border-t w-full",
    left: "left-0 border-r h-full w-3/4 sm:max-w-sm",
    right: "right-0 border-l h-full w-3/4 sm:max-w-sm",
  }

  return (
    <div
      ref={ref}
      className={cn(
        "fixed z-50 gap-4 bg-background p-6 shadow-lg transition ease-in-out",
        sideClasses[side],
        className
      )}
      {...props}
    />
  )
})
SheetContent.displayName = "SheetContent"

const SheetHeader = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn("flex flex-col space-y-2 text-center sm:text-left", className)}
    {...props}
  />
))
SheetHeader.displayName = "SheetHeader"

const SheetTitle = React.forwardRef<
  HTMLHeadingElement,
  React.HTMLAttributes<HTMLHeadingElement>
>(({ className, ...props }, ref) => (
  <h2
    ref={ref}
    className={cn("text-lg font-semibold text-foreground", className)}
    {...props}
  />
))
SheetTitle.displayName = "SheetTitle"

const SheetDescription = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLParagraphElement>
>(({ className, ...props }, ref) => (
  <p
    ref={ref}
    className={cn("text-sm text-muted-foreground", className)}
    {...props}
  />
))
SheetDescription.displayName = "SheetDescription"

export { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription }
