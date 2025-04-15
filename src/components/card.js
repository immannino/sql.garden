import * as React from "npm:react";

export function Card({title, children} = {}) {
        return (
          <div className="card">
            {title ? <h2>{title}</h2> : null}
            {children}
          </div>
        );
}