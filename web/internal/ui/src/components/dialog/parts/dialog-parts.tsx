"use client";
import * as React from "react";
import type { PropsWithChildren } from "react";
import { cn } from "../../../lib/utils";
import {
  DialogDescription as ShadcnDialogDescription,
  DialogFooter as ShadcnDialogFooter,
  DialogHeader as ShadcnDialogHeader,
  DialogTitle as ShadcnDialogTitle,
} from "../dialog";

type DefaultDialogHeaderProps = {
  title: string;
  subTitle?: string;
  className?: string;
};

export const DefaultDialogHeader = ({ title, subTitle, className }: DefaultDialogHeaderProps) => {
  return (
    <ShadcnDialogHeader
      className={cn(
        "border-b border-gray-4 dark:border-gray-900 bg-white dark:bg-black",
        className,
      )}
    >
      <ShadcnDialogTitle
        className={cn(
          "px-6 text-gray-12 font-medium text-base",
          subTitle ? "pt-4" : "py-4",
        )}
      >
        <span className="leading-[32px] text-black dark:text-gray-200">{title}</span>
      </ShadcnDialogTitle>
      <ShadcnDialogDescription
        className={cn(
          "text-gray-9 leading-[20px] text-[13px] font-normal",
          subTitle ? "px-6 pb-4 mt-0!" : "sr-only",
        )}
      >
        {subTitle || `${title} dialog`}
      </ShadcnDialogDescription>
    </ShadcnDialogHeader>
  );
};

type DefaultDialogContentAreaProps = PropsWithChildren<{
  className?: string;
}>;

export const DefaultDialogContentArea = ({
  children,
  className,
}: DefaultDialogContentAreaProps) => {
  return (
    <div
      className={cn(
        "bg-grayA-2 flex flex-col gap-4 py-4 px-6 text-gray-11 overflow-y-auto scrollbar-hide grow",
        className,
      )}
    >
      {children}
    </div>
  );
};

type DefaultDialogFooterProps = PropsWithChildren<{
  className?: string;
}>;

export const DefaultDialogFooter = ({ children, className }: DefaultDialogFooterProps) => {
  return (
    <ShadcnDialogFooter
      className={cn(
        "p-6 border-t border-gray-4 dark:border-gray-900 bg-white dark:bg-black text-gray-9",
        className,
      )}
    >
      {children}
    </ShadcnDialogFooter>
  );
};
