import * as React from "react";

import { cn } from "@/lib/utils";

type TableElement = React.TableHTMLAttributes<HTMLTableElement>;
type TableSection = React.HTMLAttributes<HTMLTableSectionElement>;
type TableRowElement = React.HTMLAttributes<HTMLTableRowElement>;
type TableHeaderCell = React.ThHTMLAttributes<HTMLTableCellElement>;
type TableDataCell = React.TdHTMLAttributes<HTMLTableCellElement>;
type TableCaptionElement = React.HTMLAttributes<HTMLTableCaptionElement>;

export const Table = React.forwardRef<HTMLTableElement, TableElement>(
  ({ className, ...props }, ref) => (
    <div className="relative w-full overflow-x-auto">
      <table
        ref={ref}
        className={cn(
          "w-full caption-bottom text-left text-sm text-zinc-700 dark:text-zinc-300",
          className,
        )}
        {...props}
      />
    </div>
  ),
);
Table.displayName = "Table";

export const TableHeader = React.forwardRef<HTMLTableSectionElement, TableSection>(
  ({ className, ...props }, ref) => (
    <thead
      ref={ref}
      className={cn("text-xs uppercase tracking-[0.2em] text-zinc-500 dark:text-zinc-400", className)}
      {...props}
    />
  ),
);
TableHeader.displayName = "TableHeader";

export const TableBody = React.forwardRef<HTMLTableSectionElement, TableSection>(
  ({ className, ...props }, ref) => (
    <tbody
      ref={ref}
      className={cn("divide-y divide-zinc-200/80 dark:divide-zinc-800/60", className)}
      {...props}
    />
  ),
);
TableBody.displayName = "TableBody";

export const TableFooter = React.forwardRef<HTMLTableSectionElement, TableSection>(
  ({ className, ...props }, ref) => (
    <tfoot
      ref={ref}
      className={cn("bg-zinc-100/60 font-medium dark:bg-zinc-900/40", className)}
      {...props}
    />
  ),
);
TableFooter.displayName = "TableFooter";

export const TableRow = React.forwardRef<HTMLTableRowElement, TableRowElement>(
  ({ className, ...props }, ref) => (
    <tr
      ref={ref}
      className={cn(
        "transition-colors hover:bg-zinc-100/60 dark:hover:bg-zinc-800/40 data-[state=selected]:bg-emerald-50 dark:data-[state=selected]:bg-zinc-900",
        className,
      )}
      {...props}
    />
  ),
);
TableRow.displayName = "TableRow";

export const TableHead = React.forwardRef<HTMLTableCellElement, TableHeaderCell>(
  ({ className, ...props }, ref) => (
    <th
      ref={ref}
      className={cn("px-4 py-3 text-left text-xs font-medium text-zinc-500 dark:text-zinc-400", className)}
      {...props}
    />
  ),
);
TableHead.displayName = "TableHead";

export const TableCell = React.forwardRef<HTMLTableCellElement, TableDataCell>(
  ({ className, ...props }, ref) => (
    <td
      ref={ref}
      className={cn("px-4 py-4 align-middle text-sm text-zinc-700 dark:text-zinc-300", className)}
      {...props}
    />
  ),
);
TableCell.displayName = "TableCell";

export const TableCaption = React.forwardRef<
  HTMLTableCaptionElement,
  TableCaptionElement
>(({ className, ...props }, ref) => (
  <caption
    ref={ref}
    className={cn("mt-4 text-sm text-zinc-500 dark:text-zinc-400", className)}
    {...props}
  />
));
TableCaption.displayName = "TableCaption";
