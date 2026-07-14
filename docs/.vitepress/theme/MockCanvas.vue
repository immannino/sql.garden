<template>
  <div class="sg-canvas-wrap" aria-hidden="true">
    <!-- Query node -->
    <div class="sg-node sg-node--query">
      <div class="sg-node-header">
        <span class="sg-node-dot" style="background:#18b569"></span>
        monthly_revenue
      </div>
      <div class="sg-node-body">
        <div class="sg-code">
          <span class="sg-kw">SELECT</span> month,<br>
          &nbsp;&nbsp;<span class="sg-kw">SUM</span>(amount) <span class="sg-kw">AS</span> revenue<br>
          <span class="sg-kw">FROM</span> orders<br>
          <span class="sg-kw">GROUP BY</span> month
        </div>
        <div class="sg-node-footer">
          <span class="sg-badge">12 rows</span>
          <span class="sg-run">▶ Run</span>
        </div>
      </div>
    </div>

    <!-- Data node -->
    <div class="sg-node sg-node--data">
      <div class="sg-node-header">
        <svg viewBox="0 0 14 14" fill="none" width="12" height="12">
          <ellipse cx="7" cy="3.5" rx="4.5" ry="1.5" stroke="currentColor" stroke-width="1.1"/>
          <path d="M2.5 3.5v7c0 .83 2.01 1.5 4.5 1.5s4.5-.67 4.5-1.5v-7" stroke="currentColor" stroke-width="1.1"/>
          <path d="M2.5 7c0 .83 2.01 1.5 4.5 1.5S11.5 7.83 11.5 7" stroke="currentColor" stroke-width="1.1" opacity=".5"/>
        </svg>
        orders
      </div>
      <div class="sg-node-body sg-cols">
        <div class="sg-col"><span class="sg-type">i32</span> order_id</div>
        <div class="sg-col"><span class="sg-type">str</span> customer</div>
        <div class="sg-col"><span class="sg-type">f64</span> amount</div>
        <div class="sg-col"><span class="sg-type">ts</span>&nbsp; created_at</div>
      </div>
      <div class="sg-node-footer">
        <span class="sg-badge">48,291 rows</span>
      </div>
    </div>

    <!-- Chart node -->
    <div class="sg-node sg-chart-node sg-node--chart">
      <div class="sg-node-header">
        <span class="sg-node-dot" style="background:#3fb950"></span>
        Revenue by Month
      </div>
      <div class="sg-bars">
        <div class="sg-bar" style="height:32px"></div>
        <div class="sg-bar" style="height:48px"></div>
        <div class="sg-bar" style="height:38px"></div>
        <div class="sg-bar" style="height:56px"></div>
        <div class="sg-bar" style="height:44px"></div>
        <div class="sg-bar" style="height:64px"></div>
        <div class="sg-bar" style="height:52px"></div>
      </div>
    </div>

    <!-- Desktop connectors -->
    <svg class="sg-connector sg-connector--desktop" viewBox="0 0 480 320" fill="none">
      <path d="M 350 75 C 260 75 220 80 164 100" stroke="#18b569" stroke-width="1" stroke-dasharray="4 3" opacity=".4"/>
      <path d="M 55 200 C 90 222 100 250 150 250" stroke="#18b569" stroke-width="1" stroke-dasharray="4 3" opacity=".4"/>
    </svg>

    <!-- Mobile connectors — vertical, Data → Query → Chart -->
    <svg class="sg-connector sg-connector--mobile" viewBox="0 0 480 455" preserveAspectRatio="none" fill="none">
      <path d="M 240 155 L 240 173" stroke="#18b569" stroke-width="1" stroke-dasharray="4 3" opacity=".4"/>
      <path d="M 240 315 L 240 335" stroke="#18b569" stroke-width="1" stroke-dasharray="4 3" opacity=".4"/>
    </svg>
  </div>
</template>
